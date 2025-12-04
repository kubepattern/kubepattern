package it.sigemi.utils;

import it.sigemi.domain.entities.pattern.K8sPattern;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Arrays;
import java.util.stream.Collectors;

public class PatternIdGenerator {

    public static String generateHash(K8sPattern pattern) {
        try {
            MessageDigest digest = MessageDigest.getInstance("MD5");

            // Ordina le risorse per consistenza
            String resourcesStr = Arrays.stream(pattern.getResources())
                    .sorted((r1, r2) -> {
                        int cmp = r1.getResource().getName().compareTo(r2.getResource().getName());
                        if (cmp != 0) return cmp;
                        String ns1 = r1.getNamespace() != null ? r1.getNamespace() : "";
                        String ns2 = r2.getNamespace() != null ? r2.getNamespace() : "";
                        return ns1.compareTo(ns2);
                    })
                    .map(r -> r.getResource().getName() + ":" + (r.getNamespace() != null ? r.getNamespace() : ""))
                    .collect(Collectors.joining("|"));

            String toHash = pattern.getMetadata().getName() + "|" + resourcesStr;

            byte[] hash = digest.digest(toHash.getBytes(StandardCharsets.UTF_8));

            // Converti in hex (primi 8 caratteri)
            StringBuilder sb = new StringBuilder();
            for (int i = 0; i < Math.min(4, hash.length); i++) {
                sb.append(String.format("%02x", hash[i]));
            }
            return sb.toString();

        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("MD5 not available", e);
        }
    }
}
