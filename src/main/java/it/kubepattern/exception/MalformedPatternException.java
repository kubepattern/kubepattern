package it.kubepattern.exception;

public class MalformedPatternException extends IllegalArgumentException {
    public MalformedPatternException(String message) {
        super("Error Parsing Pattern: " + message);

    }
}
