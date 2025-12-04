package it.sigemi.exception;

public class MalformedPatternException extends IllegalArgumentException {
    public MalformedPatternException(String message) {
        super("Error Parsing Pattern: " + message);

    }
}
