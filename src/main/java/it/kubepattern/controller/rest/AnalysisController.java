package it.kubepattern.controller.rest;

import it.kubepattern.application.service.pattern.AnalysisOrchestratorService;
import jakarta.websocket.server.PathParam;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

@Slf4j
@RestController
@RequestMapping("/analysis")
@AllArgsConstructor
public class AnalysisController {

    AnalysisOrchestratorService analysisOrchestratorService;

    @PostMapping("/cluster")
    public String analyzeCluster(@PathParam("pattern") String pattern) throws Exception {
        String message;

        if(pattern == null || pattern.isEmpty()) {
            message = "Received request to analyze cluster all patterns";
            analysisOrchestratorService.startClusterAnalysis();
        }else{
            message="Received request to analyze cluster for pattern: " + pattern;
            analysisOrchestratorService.startClusterAnalysis(pattern);
        }

        printMessage(message);

        return message + "\n";
    }

    //namespace analysis
    @PostMapping("/namespace/{namespace}")
    public String analyzeNamespace(@PathVariable String namespace, @PathParam("pattern") String pattern) throws Exception {
        String message;

        if(pattern == null || pattern.isEmpty()) {
            message = "Received request to analyze namespace " + namespace +" all patterns";
            analysisOrchestratorService.startNamespaceAnalysis(namespace);
        }else{
            message="Received request to analyze namespace "+namespace+" for pattern: " + pattern;
            analysisOrchestratorService.startNamespaceAnalysis(namespace);
        }

        printMessage(message);

        return message + "\n";
    }

    public void printMessage(String message){
        log.info("Message : {}", message);
    }
}