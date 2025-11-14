package it.sigemi.repository.pattern_as_code;

import it.sigemi.application.configuration.AppConfig;
import it.sigemi.domain.entities.pattern.PatternDefinition;
import it.sigemi.utils.PatternDefinitionLinter;
import it.sigemi.utils.PatternDefinitionParser;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Repository;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

@Slf4j
@Getter
@Setter
@Repository
@AllArgsConstructor
public class GitHubPatternDefinitionRepository implements IPatternDefinitionRepository {
    private final String gitBaseUrl;
    private final String gitToken;
    private final AppConfig appConfig;

    @Override
    public PatternDefinition getPatternDefinitionByName(String name) throws IOException, InterruptedException {
        String fileUrl = gitBaseUrl;
        if (!fileUrl.endsWith("/")) {
            fileUrl += "/";
        }

        fileUrl += name + ".json";

        String rawUrl = fileUrl;
        String content = downloadContent(rawUrl);
        log.info(content);
        return PatternDefinitionParser.fromJson(content);
    }

    public PatternDefinition getPatternDefinitionByUrl(String url) throws IOException, InterruptedException {
        String content = downloadContent(url);
        log.info(content);
        try {
            PatternDefinitionLinter.lint(content);
        }catch (Exception e){
            return null;
        }

        return PatternDefinitionParser.fromJson(content);
    }

    private String downloadContent(String url) throws IOException, InterruptedException {

        HttpRequest.Builder requestBuilder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(Duration.ofSeconds(10))
                .GET();

        if (gitToken != null && !gitToken.isEmpty()) {
            requestBuilder.header("Authorization", "Bearer " + gitToken);
        }

        requestBuilder.header("Accept", "application/vnd.github.v3+json");

        HttpRequest request = requestBuilder.build();
        HttpClient client = HttpClient.newBuilder().version(HttpClient.Version.HTTP_2)
                .connectTimeout(Duration.ofSeconds(10)).build();
        HttpResponse<String> response = client.send(
                request,
                HttpResponse.BodyHandlers.ofString());

        client.close();

        if (response.statusCode() != 200) {
            throw new IOException("HTTP error: " + response.statusCode() + " - " + response.body());
        }

        return response.body();
    }

    @Override
    public List<PatternDefinition> getAllPatternDefinitions() throws IOException, InterruptedException {
        String url = appConfig.getPatternRegistry().getUrl().concat("?ref=test");
        ArrayList<PatternDefinition> definitions = new ArrayList<>();

        String content = downloadContent(url);
        ObjectMapper objectMapper = new ObjectMapper();
        JsonNode files = objectMapper.readTree(content);
        for (JsonNode file : files) {
            String fileUrl = file.get("download_url").asText();

            PatternDefinition definition = getPatternDefinitionByUrl(fileUrl);
            definitions.add(definition);

        }

        log.info("Loaded {} pattern definitions from GitHub repository.", definitions.size());
        return definitions;
    }
}