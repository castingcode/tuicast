#!/usr/bin/env python3
import json
import sys
import threading
import time


def screen(revision=3):
    return {
        "width": 5,
        "height": 1,
        "revision": revision,
        "text": "READY",
        "cursor": {"column": 0, "row": 0, "visible": True},
        "cells": [
            {"text": c, "width": 1, "foreground": -1, "background": -1, "attributes": 0}
            for c in "READY"
        ],
    }


lock = threading.Lock()


def send(value):
    with lock:
        print(json.dumps(value), flush=True)


for line in sys.stdin:
    request = json.loads(line)
    identifier, method = request["id"], request["method"]
    response = {"jsonrpc": "2.0", "id": identifier}
    if method == "driver.ping":
        response["result"] = {"protocolVersion": "1"}
    elif method == "driver.shutdown":
        response["result"] = {"shuttingDown": True}
        send(response)
        break
    elif method == "connection.open":
        response["result"] = {"connectionId": 11}
    elif method == "session.open":
        response["result"] = {"sessionId": 22}
    elif method in ("connection.close", "session.close"):
        response["result"] = {"closed": True}
    elif method == "session.screen":
        response["result"] = screen()
        threading.Thread(
            target=lambda request_id=identifier, reply=response: (
                time.sleep(0.01 if request_id % 2 else 0),
                send(reply),
            )
        ).start()
        continue
    elif method in ("session.send", "session.press", "session.resize"):
        response["result"] = {"sent": True}
    elif method == "session.waitForIdle":
        response["result"] = screen()
    elif method == "session.wait":
        if request["params"]["matcher"] == {"contains": "MISSING"}:
            response["error"] = {
                "code": -32000,
                "message": "timed out",
                "data": {"kind": "timeout", "expected": "MISSING", "screen": screen()},
            }
        else:
            response["result"] = screen()
    elif method in ("session.subscribe", "session.subscribeEvents"):
        response["result"] = {"subscriptionId": 33}
        send(response)
        if method == "session.subscribe":
            for rev in range(1, 4):
                send(
                    {
                        "jsonrpc": "2.0",
                        "method": "session.screen",
                        "params": {"subscriptionId": 33, "sessionId": 22, "screen": screen(rev)},
                    }
                )
        else:
            for seq, typ in ((1, "bell"), (2, "enquiry")):
                send(
                    {
                        "jsonrpc": "2.0",
                        "method": "session.event",
                        "params": {
                            "subscriptionId": 33,
                            "sessionId": 22,
                            "event": {"sequence": seq, "type": typ},
                        },
                    }
                )
        continue
    elif method == "session.unsubscribe":
        response["result"] = {"unsubscribed": True}
    elif method == "hang":
        continue
    else:
        response["error"] = {"code": -32601, "message": "missing"}
    send(response)
