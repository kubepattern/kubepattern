package it.sigemi.domain.entities.cluster.relationships;

public enum RelationshipType {
    OWNS,           // Deployment -> ReplicaSet //ok
    MANAGES,        // ReplicaSet -> Pod //ok
    MOUNTS,         // Pod -> Volume //ok
    EXPOSES,        // Service -> Pod //ok
    USES_CONFIG,     // ConfigMap/Secret reference
    SAME_NETWORK,        // Network policies //
    IS_NAMESPACE_OF, // Namespace -> Generic Resource //ok
    USES_SA,        // Pod -> ServiceAccount
    HAS_AFFINITY_TO, // Pod -> Node
    REFERENCES_KRATEO,
    MANAGES_KRATEO
}