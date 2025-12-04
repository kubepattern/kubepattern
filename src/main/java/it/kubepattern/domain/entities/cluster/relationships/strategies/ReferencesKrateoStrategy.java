package it.sigemi.domain.entities.cluster.relationships.strategies;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import it.sigemi.domain.entities.cluster.K8sCluster;
import it.sigemi.domain.entities.cluster.relationships.K8sRelationship;
import it.sigemi.domain.entities.cluster.K8sResource;
import lombok.extern.slf4j.Slf4j;

import static it.sigemi.domain.entities.cluster.relationships.RelationshipType.REFERENCES_KRATEO;

@Slf4j
public class ReferencesKrateoStrategy implements RelationshipStrategy {
    ObjectMapper mapper = new ObjectMapper();
    @Override
    public void analyze(K8sCluster cluster) {
        //tables
        checkWidget(cluster, "Panel", "Table", "tables");
        checkWidget(cluster, "Column", "Table", "tables");
        checkWidget(cluster, "Row", "Table", "tables");
        checkWidget(cluster, "Page", "Table", "tables");
        //markdowns
        checkWidget(cluster, "Panel",  "Markdown", "markdowns");
        checkWidget(cluster, "Column", "Markdown", "markdowns");
        checkWidget(cluster, "Row", "Markdown", "markdowns");
        checkWidget(cluster, "Page", "Markdown", "markdowns");
        //rows
        checkWidget(cluster, "Panel",  "Row", "rows");
        checkWidget(cluster, "Column",  "Row", "rows");
        checkWidget(cluster, "Row",  "Row", "rows");
        checkWidget(cluster, "Page",  "Row", "rows");
        //columns
        checkWidget(cluster, "Panel",  "Column", "columns");
        checkWidget(cluster, "Column",  "Column", "columns");
        checkWidget(cluster, "Row",  "Column", "columns");
        checkWidget(cluster, "Page",  "Column", "columns");
        //panels
        checkWidget(cluster, "Panel",  "Panel", "panels");
        checkWidget(cluster, "Column",  "Panel", "panels");
        checkWidget(cluster, "Row",  "Panel", "panels");
        checkWidget(cluster, "Page",  "Panel", "panels");
        //navmenuitems
        checkWidget(cluster, "NavMenuItem",  "Page", "pages");
        //filters
        checkWidget(cluster, "Panel",  "Filter", "filters");
        checkWidget(cluster, "Row",  "Filter", "filters");
        checkWidget(cluster, "Column",  "Filter", "filters");
        checkWidget(cluster, "Page",  "Filter", "filters");
        //charts
        checkWidget(cluster, "Panel",  "PieChart", "piecharts");
        checkWidget(cluster, "Row",  "PieChart", "piecharts");
        checkWidget(cluster, "Column",  "PieChart", "piecharts");
        //linecharts
        checkWidget(cluster, "Panel",  "LineChart", "linecharts");
        checkWidget(cluster, "Row",  "LineChart", "linecharts");
        checkWidget(cluster, "Column",  "LineChart", "linecharts");
        //barcharts
        checkWidget(cluster, "Panel",  "BarChart", "barcharts");
        checkWidget(cluster, "Row",  "BarChart", "barcharts");
        checkWidget(cluster, "Column",  "BarChart", "barcharts");
    }

    @Override
    public boolean involve(K8sResource resource) {
        return resource.getKind().equals("Panel") || resource.getKind().equals("Table");
    }

    public void checkWidget(K8sCluster cluster, String fromKind, String toKind, String toPlural) {
        for(K8sResource panel : cluster.getResourcesByKind(fromKind)){
            String jsonString = panel.getJsonObject();
            if (jsonString == null || jsonString.isEmpty()) {
                continue;
            }
            try {
                JsonNode rootNode = mapper.readTree(jsonString);
                if(rootNode.has("raw")) {
                    rootNode = rootNode.path("raw");
                }

                JsonNode itemsNode = rootNode.path("spec").path("resourcesRefs").path("items");
                if (itemsNode.isArray()) {
                    for (JsonNode item : itemsNode) {
                        String targetName = item.path("name").asText(null);
                        String targetNamespace = item.path("namespace").asText(null);
                        String targetResourceKindPlural = item.path("resource").asText(null);

                        if (targetName != null && targetNamespace != null && targetResourceKindPlural != null) {
                            if(targetResourceKindPlural.equals(toPlural)){
                                for (K8sResource to : cluster.getResourcesByKind(toKind)) {
                                    if(to.getName().equals(targetName) && to.getNamespace().equals(targetNamespace)){
                                        cluster.addRelationship(panel, to, new K8sRelationship(REFERENCES_KRATEO));
                                    }
                                }
                            }
                        }
                    }
                }
            }catch (Exception e){
                log.info(e.getMessage());
            }
        }
    }
}