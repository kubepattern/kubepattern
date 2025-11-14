package it.sigemi.controller.rest;

import it.sigemi.application.service.pattern.PatternDefinitionLinterService;
import it.sigemi.dto.KubePatternHttpResponse;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@Slf4j
@RestController
@RequiredArgsConstructor
public class PatternController {

    private final PatternDefinitionLinterService linterService;

    @PostMapping(value = "/pattern/lint",
            consumes = MediaType.APPLICATION_JSON_VALUE,
            produces = MediaType.APPLICATION_JSON_VALUE)
    public KubePatternHttpResponse lintPattern(@RequestBody String body) {
        log.info("Received request to lint pattern");

        return linterService.lint(body);
    }
}