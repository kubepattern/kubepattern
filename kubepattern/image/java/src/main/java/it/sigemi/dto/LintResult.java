package it.sigemi.dto;

public class LintResult {
    private final boolean valid;
    private final String message;
    private final String error;

    public LintResult(boolean valid, String message, String error) {
        this.valid = valid;
        this.message = message;
        this.error = error;
    }

    public boolean isValid() {
        return valid;
    }

    public String getMessage() {
        return message;
    }

    public String getError() {
        return error;
    }
}
