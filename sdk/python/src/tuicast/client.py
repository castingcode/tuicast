from __future__ import annotations

import base64
import json
import os
import queue
import subprocess
import threading
from collections.abc import Iterator, Mapping
from dataclasses import dataclass
from typing import Any, BinaryIO, Generic, Literal, TextIO, TypeVar, overload

from .model import Key, Matcher, Modifier, Screen, Terminal, TerminalEvent, contains, not_

PROTOCOL_VERSION = "1"


class TUICastError(Exception):
    """Base SDK error."""


class ProtocolError(TUICastError):
    """Malformed or incompatible driver protocol."""


class RPCTimeoutError(TimeoutError, TUICastError):
    """A client-side RPC deadline expired."""


class RPCError(TUICastError):
    def __init__(self, code: int, message: str, data: Any = None):
        super().__init__(f"driver error {code}: {message}")
        self.code, self.message, self.data = code, message, data


class WaitError(RPCError):
    def __init__(self, code: int, message: str, data: Mapping[str, Any]):
        super().__init__(code, message, data)
        self.kind = str(data.get("kind", ""))
        self.expected = str(data.get("expected", ""))
        self.last_screen = Screen.from_dict(data["screen"])


def is_timeout(error: BaseException) -> bool:
    return isinstance(error, (TimeoutError, RPCTimeoutError)) or (
        isinstance(error, WaitError) and error.kind == "timeout"
    )


@dataclass(frozen=True, slots=True)
class Telnet:
    address: str
    timeout: float | None = None

    def params(self, default_timeout: float) -> tuple[dict[str, Any], float]:
        if not self.address:
            raise ValueError("configuring Telnet connection: address is required")
        timeout = _positive(self.timeout or default_timeout, "connection timeout")
        return {
            "protocol": "telnet",
            "address": self.address,
            "connectTimeoutMilliseconds": _milliseconds(timeout),
        }, timeout


@dataclass(frozen=True, slots=True)
class SSH:
    address: str
    username: str
    password: str = ""
    private_key: str = ""
    private_key_passphrase: str = ""
    known_hosts_file: str = ""
    host_key_fingerprint: str = ""
    insecure_skip_host_key_check: bool = False
    timeout: float | None = None

    def params(self, default_timeout: float) -> tuple[dict[str, Any], float]:
        if not self.address or not self.username:
            raise ValueError("configuring SSH connection: address and username are required")
        if not self.password and not self.private_key:
            raise ValueError("configuring SSH connection: password or private key is required")
        checks = sum(
            bool(v)
            for v in (
                self.known_hosts_file,
                self.host_key_fingerprint,
                self.insecure_skip_host_key_check,
            )
        )
        if checks != 1:
            raise ValueError(
                "configuring SSH connection: exactly one host-key verification option is required"
            )
        timeout = _positive(self.timeout or default_timeout, "connection timeout")
        return {
            "protocol": "ssh",
            "address": self.address,
            "username": self.username,
            "password": self.password,
            "privateKey": self.private_key,
            "privateKeyPassphrase": self.private_key_passphrase,
            "knownHostsFile": self.known_hosts_file,
            "hostKeyFingerprint": self.host_key_fingerprint,
            "insecureSkipHostKeyCheck": self.insecure_skip_host_key_check,
            "connectTimeoutMilliseconds": _milliseconds(timeout),
        }, timeout


class Driver:
    def __init__(self, process: subprocess.Popen[bytes], *, default_timeout: float = 30.0):
        self._process, self.default_timeout = process, _positive(default_timeout, "default timeout")
        self._lock, self._write_lock = threading.Lock(), threading.Lock()
        self._pending: dict[int, queue.Queue[Any]] = {}
        self._subscriptions: dict[int, _Subscription[Any]] = {}
        self._queued: dict[int, list[tuple[str, Mapping[str, Any]]]] = {}
        self._next_id = 0
        self._failure: BaseException | None = None
        self._closed = False
        self._reader = threading.Thread(target=self._read, name="tuicast-jsonrpc", daemon=True)
        self._reader.start()

    @classmethod
    def launch(
        cls,
        driver_path: str | os.PathLike[str] = "tuicast-driver",
        *,
        args: tuple[str, ...] = (),
        environment: Mapping[str, str] | None = None,
        stderr: int | TextIO | BinaryIO | None = None,
        default_timeout: float = 30.0,
    ) -> Driver:
        env = os.environ.copy()
        if environment:
            env.update(environment)
        process = subprocess.Popen(
            [str(driver_path), *args],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=stderr,
            env=env,
        )
        driver = cls(process, default_timeout=default_timeout)
        try:
            ping = driver._call("driver.ping", timeout=default_timeout)
            version = ping.get("protocolVersion") if isinstance(ping, dict) else None
            if version != PROTOCOL_VERSION:
                raise ProtocolError(
                    f"client requires protocol version 1, driver reported {version!r}"
                )
            return driver
        except BaseException:
            driver._stop()
            raise

    def _read(self) -> None:
        assert self._process.stdout is not None
        try:
            for raw in self._process.stdout:
                try:
                    message = json.loads(raw)
                except (json.JSONDecodeError, UnicodeDecodeError) as error:
                    raise ProtocolError(f"malformed driver response: {error}") from error
                if not isinstance(message, dict) or message.get("jsonrpc") != "2.0":
                    raise ProtocolError("driver response has unsupported JSON-RPC version")
                if "id" in message:
                    identifier = message["id"]
                    if not isinstance(identifier, int) or identifier <= 0:
                        raise ProtocolError("driver response identifier must be a positive integer")
                    with self._lock:
                        pending = self._pending.pop(identifier, None)
                    if pending is not None:
                        pending.put(message)
                elif "method" in message:
                    self._notify(str(message["method"]), message.get("params", {}))
                else:
                    raise ProtocolError("driver message is neither response nor notification")
            if not self._closed:
                raise ProtocolError("driver output closed unexpectedly")
        except BaseException as error:
            self._fail(error)

    def _notify(self, method: str, params: Any) -> None:
        if method not in ("session.screen", "session.event") or not isinstance(params, dict):
            return
        identifier = params.get("subscriptionId")
        if not isinstance(identifier, int):
            return
        with self._lock:
            subscription = self._subscriptions.get(identifier)
            if subscription is None:
                queued = self._queued.setdefault(identifier, [])
                if method == "session.screen":
                    queued[:] = [(method, params)]
                elif len(queued) < 64:
                    queued.append((method, params))
        if subscription is not None:
            subscription._deliver(method, params)

    def _fail(self, error: BaseException) -> None:
        with self._lock:
            if self._failure is None:
                self._failure = error
            pending, subscriptions = (
                list(self._pending.values()),
                list(self._subscriptions.values()),
            )
            self._pending.clear()
            self._subscriptions.clear()
        for result in pending:
            result.put(error)
        for subscription in subscriptions:
            subscription._finish()

    def _call(
        self, method: str, params: Mapping[str, Any] | None = None, *, timeout: float | None = None
    ) -> Any:
        duration = _positive(timeout or self.default_timeout, "operation timeout")
        result: queue.Queue[Any] = queue.Queue(maxsize=1)
        with self._lock:
            if self._failure:
                raise TUICastError(f"calling {method}: driver failed") from self._failure
            self._next_id += 1
            identifier = self._next_id
            self._pending[identifier] = result
        request = {"jsonrpc": "2.0", "id": identifier, "method": method}
        if params is not None:
            request["params"] = params
        try:
            assert self._process.stdin is not None
            with self._write_lock:
                self._process.stdin.write(
                    json.dumps(_plain(request), separators=(",", ":")).encode() + b"\n"
                )
                self._process.stdin.flush()
        except BaseException as error:
            with self._lock:
                self._pending.pop(identifier, None)
            self._fail(error)
            raise TUICastError(f"calling {method}: writing request failed") from error
        try:
            reply = result.get(timeout=duration)
        except queue.Empty as error:
            with self._lock:
                self._pending.pop(identifier, None)
            raise RPCTimeoutError(f"calling {method} timed out after {duration:g}s") from error
        if isinstance(reply, BaseException):
            raise TUICastError(f"calling {method}: driver failed") from reply
        if "error" in reply:
            rpc = reply["error"]
            data = rpc.get("data")
            if isinstance(data, dict) and data.get("kind") and isinstance(data.get("screen"), dict):
                raise WaitError(int(rpc["code"]), str(rpc["message"]), data)
            raise RPCError(int(rpc["code"]), str(rpc["message"]), data)
        if "result" not in reply:
            raise ProtocolError(f"calling {method}: response has no result or error")
        return reply["result"]

    def connect(self, config: Telnet | SSH) -> Connection:
        if not isinstance(config, (Telnet, SSH)):
            raise TypeError("connection configuration must be SSH or Telnet")
        params, timeout = config.params(self.default_timeout)
        result = self._call("connection.open", params, timeout=timeout + 1)
        return Connection(self, int(result["connectionId"]))

    def close(self) -> None:
        with self._lock:
            if self._closed:
                return
            self._closed = True
        try:
            if self._process.poll() is None:
                self._call("driver.shutdown", timeout=self.default_timeout)
        except TUICastError:
            pass
        finally:
            self._stop()

    def _stop(self) -> None:
        if self._process.stdin:
            self._process.stdin.close()
        try:
            self._process.wait(timeout=self.default_timeout)
        except subprocess.TimeoutExpired:
            self._process.kill()
            self._process.wait()

    def __enter__(self) -> Driver:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


class Connection:
    def __init__(self, driver: Driver, identifier: int):
        self.driver, self.id, self._closed = driver, identifier, False
        self._lock = threading.Lock()

    def open_session(
        self,
        *,
        terminal: Terminal = Terminal.VT220,
        width: int = 80,
        height: int = 24,
        answerback: str = "",
    ) -> Session:
        if terminal not in Terminal:
            raise ValueError(f"unsupported terminal profile {terminal!r}")
        if width <= 0 or height <= 0:
            raise ValueError("width and height must be positive")
        if len(answerback.encode("ascii", "strict")) > 20 or any(
            not 32 <= ord(c) <= 126 for c in answerback
        ):
            raise ValueError("answerback must be at most 20 bytes of printable ASCII")
        result = self.driver._call(
            "session.open",
            {
                "connectionId": self.id,
                "terminal": str(terminal),
                "width": width,
                "height": height,
                "answerback": answerback,
            },
        )
        return Session(self, int(result["sessionId"]))

    def close(self) -> None:
        with self._lock:
            if self._closed:
                return
            self.driver._call("connection.close", {"connectionId": self.id})
            self._closed = True

    def __enter__(self) -> Connection:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


class Session:
    def __init__(self, connection: Connection, identifier: int):
        self.connection, self.id, self._closed = connection, identifier, False
        self._lock = threading.Lock()

    @property
    def driver(self) -> Driver:
        return self.connection.driver

    def type(self, text: str) -> None:
        self.driver._call("session.send", {"sessionId": self.id, "text": text})

    def send(self, data: bytes) -> None:
        self.driver._call(
            "session.send", {"sessionId": self.id, "base64": base64.b64encode(data).decode("ascii")}
        )

    def press(self, key: Key | str, *modifiers: Modifier) -> None:
        value = str(key)
        named = {str(member) for member in Key}
        if value not in named and (len(value) != 1 or not value.isprintable()):
            raise ValueError(f"unsupported key {value!r}")
        self.driver._call(
            "session.press",
            {
                "sessionId": self.id,
                "key": value,
                "modifiers": [str(modifier) for modifier in modifiers],
            },
        )

    def resize(self, width: int, height: int) -> None:
        if width <= 0 or height <= 0:
            raise ValueError("width and height must be positive")
        self.driver._call(
            "session.resize", {"sessionId": self.id, "width": width, "height": height}
        )

    def screen(self) -> Screen:
        return Screen.from_dict(self.driver._call("session.screen", {"sessionId": self.id}))

    def wait_for(
        self, matcher: Matcher, *, timeout: float | None = None, stable_for: float = 0
    ) -> Screen:
        duration = _positive(timeout or self.driver.default_timeout, "wait timeout")
        if stable_for < 0:
            raise ValueError("stable_for must not be negative")
        params = {
            "sessionId": self.id,
            "matcher": matcher,
            "timeoutMilliseconds": _milliseconds(duration),
            "stableMilliseconds": _milliseconds(stable_for) if stable_for else 0,
        }
        return Screen.from_dict(self.driver._call("session.wait", params, timeout=duration + 1))

    def wait_for_text(self, text: str, **options: float) -> Screen:
        return self.wait_for(contains(text), **options)

    def wait_for_text_gone(self, text: str, **options: float) -> Screen:
        return self.wait_for(not_(contains(text)), **options)

    def wait_for_idle(self, *, timeout: float | None = None, quiet_for: float = 0.1) -> Screen:
        duration = _positive(timeout or self.driver.default_timeout, "idle timeout")
        quiet = _positive(quiet_for, "idle quiet period")
        params = {
            "sessionId": self.id,
            "timeoutMilliseconds": _milliseconds(duration),
            "quietMilliseconds": _milliseconds(quiet),
        }
        return Screen.from_dict(
            self.driver._call("session.waitForIdle", params, timeout=duration + 1)
        )

    def subscribe(self) -> ScreenSubscription:
        return self._subscribe("session.subscribe", True)

    def subscribe_events(self) -> EventSubscription:
        return self._subscribe("session.subscribeEvents", False)

    @overload
    def _subscribe(self, method: str, screens: Literal[True]) -> ScreenSubscription: ...

    @overload
    def _subscribe(self, method: str, screens: Literal[False]) -> EventSubscription: ...

    def _subscribe(self, method: str, screens: bool) -> ScreenSubscription | EventSubscription:
        result = self.driver._call(method, {"sessionId": self.id})
        identifier = int(result["subscriptionId"])
        subscription: ScreenSubscription | EventSubscription = (
            ScreenSubscription(self.driver, identifier)
            if screens
            else EventSubscription(self.driver, identifier)
        )
        with self.driver._lock:
            self.driver._subscriptions[identifier] = subscription
            queued = self.driver._queued.pop(identifier, [])
        for notification in queued:
            subscription._deliver(*notification)
        return subscription

    def close(self) -> None:
        with self._lock:
            if self._closed:
                return
            self.driver._call("session.close", {"sessionId": self.id})
            self._closed = True

    def __enter__(self) -> Session:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


T = TypeVar("T")
_SENTINEL = object()


class _Subscription(Generic[T], Iterator[T]):
    def __init__(self, driver: Driver, identifier: int, size: int):
        self.driver, self.id = driver, identifier
        self._queue: queue.Queue[T | object] = queue.Queue(size)
        self._closed, self._lock = False, threading.Lock()

    def __iter__(self) -> _Subscription[T]:
        return self

    def __next__(self) -> T:
        value = self.get()
        return value

    def get(self, timeout: float | None = None) -> T:
        value = self._queue.get(timeout=timeout)
        if value is _SENTINEL:
            raise StopIteration
        return value  # type: ignore[return-value]

    def _finish(self) -> None:
        with self._lock:
            if self._closed:
                return
            self._closed = True
            try:
                self._queue.put_nowait(_SENTINEL)
            except queue.Full:
                try:
                    self._queue.get_nowait()
                except queue.Empty:
                    pass
                self._queue.put_nowait(_SENTINEL)

    def _deliver(self, method: str, params: Mapping[str, Any]) -> None:
        raise NotImplementedError

    def close(self) -> None:
        with self._lock:
            if self._closed:
                return
            self._closed = True
        with self.driver._lock:
            self.driver._subscriptions.pop(self.id, None)
        try:
            self.driver._call("session.unsubscribe", {"subscriptionId": self.id})
        finally:
            try:
                self._queue.put_nowait(_SENTINEL)
            except queue.Full:
                pass

    def __enter__(self) -> _Subscription[T]:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()


class ScreenSubscription(_Subscription[Screen]):
    def __init__(self, driver: Driver, identifier: int):
        super().__init__(driver, identifier, 1)

    def _deliver(self, method: str, params: Mapping[str, Any]) -> None:
        if method != "session.screen" or not isinstance(params.get("screen"), dict):
            return
        screen = Screen.from_dict(params["screen"])
        try:
            self._queue.get_nowait()
        except queue.Empty:
            pass
        self._queue.put_nowait(screen)


class EventSubscription(_Subscription[TerminalEvent]):
    def __init__(self, driver: Driver, identifier: int):
        super().__init__(driver, identifier, 64)

    def _deliver(self, method: str, params: Mapping[str, Any]) -> None:
        if method != "session.event" or not isinstance(params.get("event"), dict):
            return
        event = params["event"]
        try:
            self._queue.put_nowait(
                TerminalEvent(
                    int(event["sequence"]), str(event["type"]), str(event.get("data", ""))
                )
            )
        except queue.Full:
            self._finish()


def _positive(value: float, name: str) -> float:
    if value <= 0:
        raise ValueError(f"{name} must be positive")
    return value


def _milliseconds(seconds: float) -> int:
    return max(1, int(seconds * 1000 + 0.999999))


def _plain(value: Any) -> Any:
    if isinstance(value, Mapping):
        return {key: _plain(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [_plain(item) for item in value]
    return value
