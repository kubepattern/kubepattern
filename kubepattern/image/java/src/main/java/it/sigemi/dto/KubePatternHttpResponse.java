package it.sigemi.dto;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class KubePatternHttpResponse {

    @JsonProperty("statusCode")
    private int statusCode;

    @JsonProperty("statusMessage")
    private String statusMessage;

    @JsonProperty("body")
    private Object body;
}