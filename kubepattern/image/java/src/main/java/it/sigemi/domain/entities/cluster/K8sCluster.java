package it.sigemi.domain.entities.cluster;

import io.kubernetes.client.openapi.JSON;
import it.sigemi.domain.entities.cluster.relationships.RelationshipType;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.ToString;
import lombok.extern.slf4j.Slf4j;
import org.jgrapht.Graph;
import org.jgrapht.graph.SimpleGraph;

import java.util.HashMap;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

@Slf4j
@Getter
@ToString
@Setter
@AllArgsConstructor
public class K8sCluster {
    private final String clusterName;
    private final String clusterUrl;
    private final Graph<K8sResource, K8sRelationship> graph;

    public K8sCluster() {
        this.clusterName = "default-cluster";
        this.clusterUrl = null;
        this.graph = new SimpleGraph<>(K8sRelationship.class);
    }

    public K8sCluster(String clusterName, String clusterUrl) {
        this.clusterUrl = clusterUrl;
        this.clusterName = clusterName;
        this.graph = new SimpleGraph<>(K8sRelationship.class);
    }

    public void addResource(K8sResource resource) {
        this.graph.addVertex(resource);
    }

    public void addRelationship(K8sResource from, K8sResource to, K8sRelationship relationship) {
        relationship.setFromId(from.getUid());
        relationship.setToId(to.getUid());
        this.graph.addEdge(from, to, relationship);
    }

    public List<K8sResource> getResourcesByKind(String kind) {
        return graph.vertexSet().stream().filter(r -> r.getKind().equals(kind)).toList();
    }

    public List<K8sResource> getResourcesByNamespace(String namespace) {
        //fix cluster-scoped resources by avoiding them
        List<K8sResource> resources = new ArrayList<>();
        for(K8sResource resource : graph.vertexSet()) {
            try {
                if(resource.getNamespace() != null && resource.getNamespace().equals(namespace))
                {
                    resources.add(resource);
                }
            }catch(Exception e) {
                log.info(e.getMessage());
            }

        }
        return resources;
    }

    public List<K8sResource> getAllResources() {
        return graph.vertexSet().stream().toList();
    }

    public List<K8sResource> getNeighbours(K8sResource resource, RelationshipType type) {
        List<K8sResource> neighbours = new ArrayList<>();
        graph.incomingEdgesOf(resource).forEach(edge -> {
            if(edge.getType().equals(type)) {
                neighbours.add(graph.getEdgeTarget(edge));
                neighbours.add(graph.getEdgeSource(edge));
            }
        });
        return neighbours;
    }

    public boolean sameNeighbour(K8sResource from, K8sResource to, RelationshipType type) {
        for (K8sResource fromNeighbours : getNeighbours(from, type)) {
            for (K8sResource toNeighbours : getNeighbours(to, type)) {
                if(fromNeighbours.equals(toNeighbours) && !from.equals(to)) {
                    log.info("same neighbours from {} to {} type: {} ", from, to, type);
                    return true;
                }
            }
        }
        return false;
    }

    public String getAsJson() {
        Map<String, Object> serializableCluster = new HashMap<>();
        serializableCluster.put("clusterName", this.clusterName);
        serializableCluster.put("clusterUrl", this.clusterUrl);

        Map<String, Object> serializableGraph = new HashMap<>();

        List<Map<String, Object>> serializableVertices = this.graph.vertexSet().stream()
                .map(vertex -> {
                    Map<String, Object> serializableVertex = new HashMap<>();
                    serializableVertex.put("uid", vertex.getUid());
                    serializableVertex.put("apiVersion", vertex.getApiVersion());
                    serializableVertex.put("name", vertex.getName());
                    serializableVertex.put("kind", vertex.getKind());
                    serializableVertex.put("namespace", vertex.getNamespace());
                    //serializableVertex.put("labels", vertex.getLabels());
                    return serializableVertex;
                })
                .collect(Collectors.toList());

        serializableGraph.put("vertices", serializableVertices);

        List<Map<String, Object>> serializableEdges = this.graph.edgeSet().stream()
                .map(edge -> {
                    K8sResource source = this.graph.getEdgeSource(edge);
                    K8sResource target = this.graph.getEdgeTarget(edge);

                    Map<String, Object> serializableEdge = new HashMap<>();
                    serializableEdge.put("source", source.toString());
                    serializableEdge.put("target", target.toString());
                    serializableEdge.put("type", edge.getType());
                    return serializableEdge;
                })
                .collect(Collectors.toList());

        serializableGraph.put("edges", serializableEdges);

        serializableCluster.put("graph", serializableGraph);

        try {
            return JSON.getGson().toJson(serializableCluster);
        } catch (Exception e) {
            log.error("Fail to serialize K8sCluster in JSON", e);
            return String.format("{\"error\": \"Fail to serialize: %s\"}", e.getMessage());
        }
    }

    public boolean hasNamespace(String namespace) {
        return getResourcesByKind("Namespace").stream().anyMatch(resource -> resource.getName().equals(namespace));
    }
}
