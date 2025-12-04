package it.sigemi.exception;

import lombok.Getter;

@Getter
public class PatternDefinitionNotFoundException extends RuntimeException {
    private final String patternDefinitionName;

    public PatternDefinitionNotFoundException(String name) {
        super("Pattern definition not found: " + name);
        this.patternDefinitionName = name;
    }
}
