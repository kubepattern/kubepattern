package it.kubepattern.domain.entities.pattern;

import it.kubepattern.domain.entities.cluster.K8sPatternResource;
import lombok.*;

import java.util.Arrays;
import java.util.List;
import java.util.Objects;

@Getter
@Setter
@ToString
@Builder
@AllArgsConstructor
public class K8sPattern {
    private K8sPatternMetadata metadata;
    private String message; // ex: Consider using a sidecar container to enhance your application's functionality and maintainability.
    private List<K8sPatternScore> scores; // ex: {filters: 98, commonRelationships: 80} out of 100
    private K8sPatternResource[] resources;

    public void addScore(K8sPatternScore score) {
        scores.add(score);
    }

    public int getTotalScore() {
        return 100;
    }

    public PatternConfidence calculateConfidence() {
        int total = getTotalScore();

        if(total <= 40) {
            return PatternConfidence.LOW;
        }else if(total <= 70) {
            return PatternConfidence.MEDIUM;
        }else if(total <= 100) {
            return PatternConfidence.HIGH;
        }else {
            return PatternConfidence.LOW;
        }
    }

    public String generateMessage() {
        String message = this.message;
        for (K8sPatternResource resource : resources) {
            message = message.replace("{{" + resource.getName() + ".name}}", resource.getResource().getName());

            if(resource.getNamespace() != null) {
                message = message.replace("{{" + resource.getName() + ".namespace}}", resource.getNamespace());
            }
        }
        return message;
    }

    @Override
    public boolean equals(Object object) {
        if (this == object) return true;
        if (object == null || getClass() != object.getClass()) return false;
        K8sPattern that = (K8sPattern) object;

        return Objects.equals(metadata, that.metadata) &&
                Arrays.equals(resources, that.resources);
    }

    @Override
    public int hashCode() {
        int result = Objects.hashCode(metadata);

        result = 31 * result + Arrays.hashCode(resources);

        return result;
    }

}