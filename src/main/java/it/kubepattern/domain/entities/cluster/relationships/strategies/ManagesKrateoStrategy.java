package it.kubepattern.domain.entities.cluster.relationships.strategies;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import it.kubepattern.domain.entities.cluster.K8sCluster;
import it.kubepattern.domain.entities.cluster.relationships.K8sRelationship;
import it.kubepattern.domain.entities.cluster.K8sResource;
import lombok.extern.slf4j.Slf4j;

import static it.kubepattern.domain.entities.cluster.relationships.RelationshipType.MANAGES_KRATEO;

@Slf4j
public class ManagesKrateoStrategy implements RelationshipStrategy{
    ObjectMapper mapper = new ObjectMapper();
    @Override
    public void analyze(K8sCluster cluster) {
        for(K8sResource resource : cluster.getResourcesByKind("Markdown")){
            String jsonString = resource.getJsonObject();

            if (jsonString == null || jsonString.isEmpty()) {
                continue;
            }
            try {
                JsonNode rootNode = mapper.readTree(jsonString);
                // Naviga al nodo "raw" se presente, come visto nei log
                if(rootNode.has("raw")) {
                    rootNode = rootNode.path("raw");
                }

                //managed-by label is the key
                String managedByValue = "";
                if(rootNode.path("metadata").path("labels").has("managed-by")) {
                    managedByValue = rootNode
                            .path("metadata")
                            .path("labels")
                            .path("managed-by")
                            .asText(null);
                }else if(rootNode.path("metadata").path("labels").has("krateo.io/managed-by")) {
                    managedByValue = rootNode
                            .path("metadata")
                            .path("labels")
                            .path("krateo.io/managed-by")
                            .asText(null);
                }else if(rootNode.path("metadata").path("labels").has("krateo.io/portal")) {
                    managedByValue = rootNode
                            .path("metadata")
                            .path("labels")
                            .path("krateo.io/portal")
                            .asText(null);
                }

                boolean find = false;
                if (managedByValue != null && !managedByValue.isEmpty()) {
                    for (K8sResource restaction : cluster.getResourcesByKind("Table")) {
                        if(restaction.getName().equals(managedByValue)){
                            cluster.addRelationship(restaction, resource, new K8sRelationship(MANAGES_KRATEO));
                            find = true;
                            break;
                        }
                    }
                    if(!find) {
                        for (K8sResource restaction : cluster.getResourcesByKind("Paragraph")) {
                            if(restaction.getName().equals(managedByValue)){
                                cluster.addRelationship(restaction, resource, new K8sRelationship(MANAGES_KRATEO));
                                break;
                            }
                        }
                    }
                }

            }catch (Exception e){
                log.error("Errore nell'analisi della risorsa Markdown {}: {}", resource.getName(), e.getMessage());
            }
        }
    }

    @Override
    public boolean involve(K8sResource resource) {
        return resource.getKind().equals("Markdown") || resource.getKind().equals("Paragraph") || resource.getKind().equals("Table");
    }
}