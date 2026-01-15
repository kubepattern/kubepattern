package it.kubepattern.application.service.pattern;

import io.kubernetes.client.openapi.ApiException;
import it.kubepattern.application.service.cluster.ClusterLifecycleService;
import it.kubepattern.domain.entities.cluster.*;
import it.kubepattern.domain.entities.pattern.K8sPattern;
import it.kubepattern.domain.entities.pattern.PatternAnalysisEngine;
import it.kubepattern.domain.entities.pattern.PatternDefinition;
import it.kubepattern.exception.MalformedPatternException;
import it.kubepattern.exception.NamespaceNotFoundException;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Async;
import org.springframework.stereotype.Service;
import java.util.List;


@Slf4j
@AllArgsConstructor
@Service
public class AnalysisOrchestratorService {

    ClusterLifecycleService clusterLifecycleService;
    FetchPatternDefinitionService fetchPatternDefinitionService;
    PatternService patternService;
    PatternAnalysisEngine patternAnalysisEngine;

    @Async
    public void startNamespaceAnalysis(String namespace) throws NamespaceNotFoundException, ApiException, Exception {
        log.info("Starting namespace analysis...");
        K8sCluster cluster= clusterLifecycleService.getNamespaceAndPopulate(namespace);
        analyzePatterns(cluster);
        log.info("Namespace analysis completed.");
    }

    @Async
    public void startClusterAnalysis() throws Exception {
        K8sCluster cluster = clusterLifecycleService.getClusterAndPopulate();
        analyzePatterns(cluster);
    }

    @Async
    public void startClusterAnalysis(String patternName) throws Exception {
        K8sCluster cluster = clusterLifecycleService.getClusterAndPopulate();
        analyzePattern(cluster,  patternName);
    }

    public void analyzePatterns(K8sCluster cluster) throws Exception{
        log.info("Analyzing patterns...");
        patternService.deleteAllPatterns();

        for (PatternDefinition patternDefinition : fetchPatternDefinitionService.getAllPatternDefinitions()) {
            analyzePattern(patternDefinition, cluster);
        }
    }

    public void analyzePattern(K8sCluster cluster, String name) throws MalformedPatternException, Exception {
        log.info("Analyzing pattern {} ...", name);
        PatternDefinition patternDefinition = fetchPatternDefinitionService.fetchPatternDefinition(name);
        analyzePattern(patternDefinition, cluster);
    }

    public void analyzePattern(PatternDefinition patternDefinition, K8sCluster cluster) throws Exception {
        if(patternDefinition==null){
            log.error("pattern definition is null");
            throw new Exception("pattern definition is null");
        }else {
            List<K8sPattern> patterns = patternAnalysisEngine.analyze(patternDefinition, cluster);
            savePatterns(patterns);
        }
    }

    public void savePatterns(List<K8sPattern> patterns) throws ApiException {
        for(K8sPattern pattern: patterns) {
            log.info(pattern.toString());
            patternService.savePattern(pattern);
        }
    }
}
