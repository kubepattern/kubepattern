package it.sigemi.controller.rest;

import it.sigemi.application.service.pattern.PatternAnalysisOrchestratorService;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@Slf4j
@RestController
@RequestMapping("/analysis")
@AllArgsConstructor
public class AnalysisController {

    PatternAnalysisOrchestratorService patternAnalysisOrchestratorService;
    //cluster analysis
    @PostMapping("/cluster")
    public String analyzeCluster() throws Exception {
        log.info("Received request to analyze cluster");
        patternAnalysisOrchestratorService.startClusterAnalysis();
        return "Cluster analysis started\n";
    }

    //namespace analysis
    @PostMapping("/namespace/{namespace}")
    public String analyzeNamespace(@PathVariable String namespace) throws Exception {
        log.info("Received request to analyze namespace {}", namespace);
        patternAnalysisOrchestratorService.startNamespaceAnalysis(namespace);
        return "Namespace analysis started\n";
    }
}