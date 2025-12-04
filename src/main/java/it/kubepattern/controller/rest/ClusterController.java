package it.kubepattern.controller.rest;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.application.service.cluster.ClusterLifecycleService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@Slf4j
@RestController
public class ClusterController {
    ClusterLifecycleService clusterLifecycleService;

    ClusterController(ClusterLifecycleService clusterLifecycleService) {
        this.clusterLifecycleService = clusterLifecycleService;
    }

    @GetMapping("/cluster/graph")
    public String getClusterGraph() throws ApiException {
        log.info("Received request to get cluster graph");

        return clusterLifecycleService.getClusterAsJson();
    }
}
