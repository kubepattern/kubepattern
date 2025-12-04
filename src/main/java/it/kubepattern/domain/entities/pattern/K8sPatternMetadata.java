package it.kubepattern.domain.entities.pattern;

import lombok.*;

@Getter
@Setter
@ToString
@AllArgsConstructor
@EqualsAndHashCode
@NoArgsConstructor
public class K8sPatternMetadata {
    private String version;//ex: kubepattern.it/v1
    private String name;// ex: Sidecar, Health Probe, ...
    private String description;//ex: A sidecar is a container that runs alongside the main application...
    private String docUrl;// ex: https://git.io/patterns/sidecar
    private String gitUrl;// ex: https://git.io/patterns/sidecar
    private String category;// ex: Reliability, Security, Performance
    private PatternSeverity severity;// ex: INFO, WARNING, CRITICAL
    private PatternType type;//ex: Structural, Behavioral, ...
}
