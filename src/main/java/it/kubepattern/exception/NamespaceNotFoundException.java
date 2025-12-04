package it.kubepattern.exception;

import lombok.Getter;

@Getter
public class NamespaceNotFoundException extends RuntimeException {
    private final String namespace;
    public NamespaceNotFoundException(String namespace) {
        super("Namespace not found: " + namespace + " in cluster.");
        this.namespace = namespace;
    }
}
