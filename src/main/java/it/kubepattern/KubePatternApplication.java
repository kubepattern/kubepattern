package it.kubepattern;

import it.kubepattern.application.configuration.AppConfig;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.scheduling.annotation.EnableAsync;

@SpringBootApplication
@EnableAsync
@EnableConfigurationProperties(AppConfig.class)
public class KubePatternApplication {

	public static void main(String[] args) {
		SpringApplication.run(KubePatternApplication.class, args);
	}

}
