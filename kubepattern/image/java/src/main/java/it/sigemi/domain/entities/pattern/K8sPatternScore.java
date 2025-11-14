package it.sigemi.domain.entities.pattern;

import lombok.*;

@Getter
@Setter
@ToString
@AllArgsConstructor
public class K8sPatternScore {
    private String category;
    private String motivation;
    private int score;
    //private int weight;
}
