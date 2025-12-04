package it.kubepattern.domain.entities.filter;

public enum FilterOperator {
    EQUALS,
    NOT_EQUALS,
    GREATER_THAN,
    GREATER_OR_EQUAL,
    LESS_THAN,
    LESS_OR_EQUAL,
    EXISTS,
    NOT_EXISTS,
    IS_EMPTY,
    ARRAY_SIZE_EQUALS,
    ARRAY_SIZE_GREATER_THAN,
    ARRAY_SIZE_LESS_THAN,
    ARRAY_SIZE_GREATER_OR_EQUAL,
    ARRAY_SIZE_LESS_OR_EQUAL
}
