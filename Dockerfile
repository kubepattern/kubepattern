FROM eclipse-temurin:21-jre-alpine

WORKDIR /app

# Copia il JAR compilato da Maven
COPY target/demo-0.0.1-SNAPSHOT.jar app.jar

# Esponi la porta dell'applicazione Spring Boot
EXPOSE 8080

# Esegui l'applicazione
ENTRYPOINT ["java", "-jar", "app.jar"]
