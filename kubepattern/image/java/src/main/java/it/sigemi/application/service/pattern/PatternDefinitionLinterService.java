package it.sigemi.application.service.pattern;

import it.sigemi.dto.KubePatternHttpResponse;
import it.sigemi.dto.LintResult;
import it.sigemi.exception.MalformedPatternException;
import it.sigemi.utils.PatternDefinitionLinter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Service
@Slf4j
public class PatternDefinitionLinterService {

    public KubePatternHttpResponse lint(String patternJson) {
        try {
            PatternDefinitionLinter.lint(patternJson);

            LintResult result = new LintResult(
                    true,
                    "Pattern validation successful",
                    null
            );

            return new KubePatternHttpResponse(
                    200,
                    "OK",
                    result
            );

        } catch (MalformedPatternException e) {
            LintResult result = new LintResult(
                    false,
                    "Pattern validation failed",
                    e.getMessage()
            );

            return new KubePatternHttpResponse(
                    400,
                    "Bad Request",
                    result
            );

        } catch (Exception e) {
            LintResult result = new LintResult(
                    false,
                    "Internal error during validation",
                    e.getMessage()
            );

            return new KubePatternHttpResponse(
                    500,
                    "Internal Server Error",
                    result
            );
        }
    }

}
