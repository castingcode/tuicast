package dev.castingcode.tuicast;

/**
 * A BELL, ENQ, or unsupported-sequence event. {@code type} is {@code "bell"}, {@code "enquiry"}, or
 * {@code "unsupported"}. {@code data} is the configured answerback for an enquiry and the ignored
 * sequence, such as {@code "CSI ? 2004 h"}, for an unsupported event.
 */
public record TerminalEvent(long sequence, String type, String data) {}
