package it.kubepattern.application.configuration;

import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.*;
import io.kubernetes.client.util.Config;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.io.IOException;
import java.time.Duration;

@Configuration
@Slf4j
public class KubernetesClientConfig {
    @Bean
    public ApiClient apiClient() throws IOException {
        ApiClient client = Config.defaultClient();

        client.setHttpClient(
                client.getHttpClient().newBuilder()
                        .readTimeout(Duration.ZERO)
                        .build()
        );

        log.info("Kubernetes API client configured for: {}", client.getBasePath());
        return client;
    }

    @Bean
    public CoreV1Api coreV1Api(ApiClient apiClient) {
        return new CoreV1Api(apiClient);
    }

    @Bean
    public AppsV1Api appsV1Api(ApiClient apiClient) {
        return new AppsV1Api(apiClient);
    }

    @Bean
    public BatchV1Api batchV1Api(ApiClient apiClient) {
        return new BatchV1Api(apiClient);
    }

    @Bean
    public NetworkingV1Api networkingV1Api(ApiClient apiClient) {
        return new NetworkingV1Api(apiClient);
    }

    //storageApi
    @Bean
    public StorageV1Api storageV1Api(ApiClient apiClient) {
        return new StorageV1Api(apiClient);
    }

    @Bean
    public CustomObjectsApi customObjectsApi(ApiClient apiClient) {
        return new CustomObjectsApi(apiClient);
    }
}
