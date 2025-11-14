package it.sigemi.domain.entities.filter;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

import it.sigemi.domain.entities.cluster.K8sResource;
import io.kubernetes.client.openapi.models.V1ObjectMeta;
import io.kubernetes.client.openapi.models.V1Pod;
import io.kubernetes.client.openapi.models.V1Deployment;
import io.kubernetes.client.openapi.models.V1DeploymentSpec;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;

class ResourceFilterEngineTest {

    @Test
    void testFilterByNameEquals() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("nginx");
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        FilterRule rule = new FilterRule("$.metadata.name", FilterOperator.EQUALS, new String[]{"nginx"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("nginx", filtered.get(0).getName());
    }

    @Test
    void testFilterByReplicasGreaterOrEqual() {
        V1Deployment dep = new V1Deployment();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("my-dep-nginx");
        dep.setMetadata(meta);
        V1DeploymentSpec spec = new V1DeploymentSpec();
        spec.setReplicas(5);
        dep.setSpec(spec);

        K8sResource resource = new K8sResource(dep);

        FilterRule rule = new FilterRule("$.spec.replicas", FilterOperator.GREATER_OR_EQUAL, new String[]{"5"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("my-dep-nginx", filtered.get(0).getName());
    }

    @Test
    void testFilterByLabelExists() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("redis");
        meta.setLabels(Collections.singletonMap("vendor", "sigemi"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        FilterRule rule = new FilterRule("$.metadata.labels.vendor", FilterOperator.EXISTS, new String[]{"sigemi"});
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), Collections.singletonList(rule), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("redis", filtered.get(0).getName());
    }

    // Nuovi test

    @Test
    void testFilterByNameNotEquals() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("nginx");
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        FilterRule rule = new FilterRule("$.metadata.name", FilterOperator.NOT_EQUALS, new String[]{"apache"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("nginx", filtered.get(0).getName());
    }

    @Test
    void testFilterByNameNotEqualsExcluded() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("nginx");
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        FilterRule rule = new FilterRule("$.metadata.name", FilterOperator.NOT_EQUALS, new String[]{"nginx"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(0, filtered.size());
    }

    @Test
    void testFilterByReplicasGreaterThan() {
        V1Deployment dep = new V1Deployment();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("my-deployment");
        dep.setMetadata(meta);
        V1DeploymentSpec spec = new V1DeploymentSpec();
        spec.setReplicas(10);
        dep.setSpec(spec);

        K8sResource resource = new K8sResource(dep);

        FilterRule rule = new FilterRule("$.spec.replicas", FilterOperator.GREATER_THAN, new String[]{"5"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
    }

    @Test
    void testFilterByReplicasLessThan() {
        V1Deployment dep = new V1Deployment();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("small-deployment");
        dep.setMetadata(meta);
        V1DeploymentSpec spec = new V1DeploymentSpec();
        spec.setReplicas(2);
        dep.setSpec(spec);

        K8sResource resource = new K8sResource(dep);

        FilterRule rule = new FilterRule("$.spec.replicas", FilterOperator.LESS_THAN, new String[]{"5"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("small-deployment", filtered.get(0).getName());
    }

    @Test
    void testFilterByReplicasLessOrEqual() {
        V1Deployment dep = new V1Deployment();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("exact-deployment");
        dep.setMetadata(meta);
        V1DeploymentSpec spec = new V1DeploymentSpec();
        spec.setReplicas(3);
        dep.setSpec(spec);

        K8sResource resource = new K8sResource(dep);

        FilterRule rule = new FilterRule("$.spec.replicas", FilterOperator.LESS_OR_EQUAL, new String[]{"3"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("exact-deployment", filtered.get(0).getName());
    }

    @Test
    void testFilterByLabelNotExists() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("simple-pod");
        meta.setLabels(Collections.singletonMap("app", "web"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        FilterRule rule = new FilterRule("$.metadata.labels.environment", FilterOperator.NOT_EXISTS, new String[]{});
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), Collections.singletonList(rule), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("simple-pod", filtered.get(0).getName());
    }

    @Test
    void testFilterMatchAllMultipleRules() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("production-nginx");
        meta.setLabels(Map.of("app", "nginx", "environment", "production"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchAllRules = Arrays.asList(
                new FilterRule("$.metadata.labels.app", FilterOperator.EQUALS, new String[]{"nginx"}),
                new FilterRule("$.metadata.labels.environment", FilterOperator.EQUALS, new String[]{"production"})
        );
        ResourceFilter filter = new ResourceFilter(matchAllRules, Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("production-nginx", filtered.get(0).getName());
    }

    @Test
    void testFilterMatchAllFailsWhenOneRuleFails() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("staging-nginx");
        meta.setLabels(Map.of("app", "nginx", "environment", "staging"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchAllRules = Arrays.asList(
                new FilterRule("$.metadata.labels.app", FilterOperator.EQUALS, new String[]{"nginx"}),
                new FilterRule("$.metadata.labels.environment", FilterOperator.EQUALS, new String[]{"production"})
        );
        ResourceFilter filter = new ResourceFilter(matchAllRules, Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(0, filtered.size());
    }

    @Test
    void testFilterMatchAnyMultipleRules() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("backend-pod");
        meta.setLabels(Collections.singletonMap("tier", "backend"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchAnyRules = Arrays.asList(
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"frontend"}),
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"backend"})
        );
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), matchAnyRules, Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("backend-pod", filtered.get(0).getName());
    }

    @Test
    void testFilterMatchAnyFailsWhenAllRulesFail() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("database-pod");
        meta.setLabels(Collections.singletonMap("tier", "database"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchAnyRules = Arrays.asList(
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"frontend"}),
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"backend"})
        );
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), matchAnyRules, Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(0, filtered.size());
    }

    @Test
    void testFilterMatchNoneAllRulesFail() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("allowed-pod");
        meta.setLabels(Collections.singletonMap("status", "active"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchNoneRules = Arrays.asList(
                new FilterRule("$.metadata.labels.status", FilterOperator.EQUALS, new String[]{"deprecated"}),
                new FilterRule("$.metadata.labels.status", FilterOperator.EQUALS, new String[]{"deleted"})
        );
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), Collections.emptyList(), matchNoneRules);

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("allowed-pod", filtered.get(0).getName());
    }

    @Test
    void testFilterMatchNoneExcludesWhenOneRuleMatches() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("blocked-pod");
        meta.setLabels(Collections.singletonMap("status", "deprecated"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchNoneRules = Arrays.asList(
                new FilterRule("$.metadata.labels.status", FilterOperator.EQUALS, new String[]{"deprecated"}),
                new FilterRule("$.metadata.labels.status", FilterOperator.EQUALS, new String[]{"deleted"})
        );
        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), Collections.emptyList(), matchNoneRules);

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(0, filtered.size());
    }

    @Test
    void testCombinedMatchAllAndMatchAny() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("complex-pod");
        meta.setLabels(Map.of("app", "nginx", "tier", "frontend", "environment", "production"));
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        List<FilterRule> matchAllRules = Collections.singletonList(
                new FilterRule("$.metadata.labels.app", FilterOperator.EQUALS, new String[]{"nginx"})
        );
        List<FilterRule> matchAnyRules = Arrays.asList(
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"frontend"}),
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"backend"})
        );

        ResourceFilter filter = new ResourceFilter(matchAllRules, matchAnyRules, Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("complex-pod", filtered.get(0).getName());
    }

    @Test
    void testCombinedAllMatchTypesWithMatchNone() {
        V1Deployment dep = new V1Deployment();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("valid-deployment");
        meta.setLabels(Map.of("app", "web", "tier", "frontend"));
        dep.setMetadata(meta);
        V1DeploymentSpec spec = new V1DeploymentSpec();
        spec.setReplicas(3);
        dep.setSpec(spec);

        K8sResource resource = new K8sResource(dep);

        List<FilterRule> matchAllRules = Collections.singletonList(
                new FilterRule("$.spec.replicas", FilterOperator.GREATER_THAN, new String[]{"1"})
        );
        List<FilterRule> matchAnyRules = Arrays.asList(
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"frontend"}),
                new FilterRule("$.metadata.labels.tier", FilterOperator.EQUALS, new String[]{"backend"})
        );
        List<FilterRule> matchNoneRules = Collections.singletonList(
                new FilterRule("$.metadata.labels.status", FilterOperator.EQUALS, new String[]{"deprecated"})
        );

        ResourceFilter filter = new ResourceFilter(matchAllRules, matchAnyRules, matchNoneRules);

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("valid-deployment", filtered.get(0).getName());
    }

    @Test
    void testMultipleResourcesFiltering() {
        V1Pod pod1 = new V1Pod();
        V1ObjectMeta meta1 = new V1ObjectMeta();
        meta1.setName("pod-1");
        meta1.setLabels(Collections.singletonMap("environment", "production"));
        pod1.setMetadata(meta1);

        V1Pod pod2 = new V1Pod();
        V1ObjectMeta meta2 = new V1ObjectMeta();
        meta2.setName("pod-2");
        meta2.setLabels(Collections.singletonMap("environment", "staging"));
        pod2.setMetadata(meta2);

        V1Pod pod3 = new V1Pod();
        V1ObjectMeta meta3 = new V1ObjectMeta();
        meta3.setName("pod-3");
        meta3.setLabels(Collections.singletonMap("environment", "production"));
        pod3.setMetadata(meta3);

        List<K8sResource> resources = Arrays.asList(
                new K8sResource(pod1),
                new K8sResource(pod2),
                new K8sResource(pod3)
        );

        FilterRule rule = new FilterRule("$.metadata.labels.environment", FilterOperator.EQUALS, new String[]{"production"});
        ResourceFilter filter = new ResourceFilter(Collections.singletonList(rule), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(resources, filter);
        assertEquals(2, filtered.size());
        assertTrue(filtered.stream().anyMatch(r -> r.getName().equals("pod-1")));
        assertTrue(filtered.stream().anyMatch(r -> r.getName().equals("pod-3")));
    }

    @Test
    void testEmptyFilterReturnsAllResources() {
        V1Pod pod = new V1Pod();
        V1ObjectMeta meta = new V1ObjectMeta();
        meta.setName("test-pod");
        pod.setMetadata(meta);

        K8sResource resource = new K8sResource(pod);

        ResourceFilter filter = new ResourceFilter(Collections.emptyList(), Collections.emptyList(), Collections.emptyList());

        List<K8sResource> filtered = ResourceFilterEngine.getFilteredResources(Collections.singletonList(resource), filter);
        assertEquals(1, filtered.size());
        assertEquals("test-pod", filtered.get(0).getName());
    }
}
