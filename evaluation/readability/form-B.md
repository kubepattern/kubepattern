# Sondaggio sulla leggibilità di regole per Kubernetes (modulo B)

Grazie per il tuo tempo. Il sondaggio confronta due sintassi per scrivere la stessa regola su un cluster Kubernetes. Si valutano le sintassi, non chi risponde.

**Istruzioni**
- Durata stimata: 30–40 minuti.
- Prima di iniziare leggi il primer (`primer.md`), che spiega entrambe le sintassi. Puoi riaprirlo quando vuoi.
- Non usare altri strumenti: niente ricerche sul web, niente assistenti AI, niente cluster.
- Procedi in ordine e non modificare le risposte delle parti già completate.

## Parte 0: esperienza

**B1** Da quanto usi Kubernetes?  ☐ mai  ☐ meno di 1 anno  ☐ 1–3 anni  ☐ più di 3 anni

**B2** Esperienza con Kyverno o con CEL:  ☐ nessuna  ☐ ho letto qualche policy o espressione  ☐ ne ho scritte alcune  ☐ le uso regolarmente

**B3** Esperienza con KubePattern:  ☐ nessuna  ☐ ne ho sentito parlare  ☐ l'ho usato

**B4** Quali di questi progetti conosci?  ☐ CloudNativePG  ☐ Argo CD  ☐ cert-manager  ☐ Crossplane

## Parte 1: comprensione

Per ogni esempio vedi una regola, scritta in una sola delle due sintassi, e un piccolo stato del cluster. I nomi e i messaggi delle regole sono volutamente neutri. Segna l'ora prima di iniziare ogni esempio.

### Esempio 1 (sintassi: Kyverno)

**Contesto.** CloudNativePG gestisce database PostgreSQL su Kubernetes. Un `Cluster` è un database PostgreSQL; uno `ScheduledBackup` pianifica i backup periodici di un database.

**Regola.**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: rule-e1
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["postgresql.cnpg.io"]
        apiVersions: ["v1"]
        resources: ["clusters"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: schedules
      expression: globalContext.Get("scheduledbackups.postgresql.cnpg.io", "")
  validations:
    - expression: >-
        dyn(variables.schedules).exists(d, d.metadata.namespace == object.metadata.namespace
          && d.spec.cluster.name == object.metadata.name)
      messageExpression: "'Finding on ' + object.metadata.name"
```

**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-nobackup
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-sb-elsewhere
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-renamed-sb
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-typo
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-two-sb
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: db-sb-elsewhere
  namespace: kp-db-b
spec:
  cluster:
    name: db-sb-elsewhere
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: nightly
  namespace: kp-db-a
spec:
  cluster:
    name: db-renamed-sb
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: db-typo
  namespace: kp-db-a
spec:
  cluster:
    name: db-typ0
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: db-two-sb-hourly
  namespace: kp-db-a
spec:
  cluster:
    name: db-two-sb
---
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: db-two-sb-weekly
  namespace: kp-db-a
spec:
  cluster:
    name: db-two-sb
```

**E1.1** Descrivi in una frase che cosa segnala questa regola.

> 

**E1.2** Quali oggetti vengono segnalati dalla regola?

| Oggetto | Segnalato? |
|---|---|
| Cluster `kp-db-a/db-nobackup` | ☐ sì  ☐ no |
| Cluster `kp-db-a/db-sb-elsewhere` | ☐ sì  ☐ no |
| Cluster `kp-db-a/db-renamed-sb` | ☐ sì  ☐ no |
| Cluster `kp-db-a/db-typo` | ☐ sì  ☐ no |
| Cluster `kp-db-a/db-two-sb` | ☐ sì  ☐ no |

**E1.3** Quanto sei sicuro delle tue risposte?  ☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)

**E1.4** Quanti minuti hai impiegato per questo esempio?  ____

### Esempio 2 (sintassi: KubePattern)

**Contesto.** In CloudNativePG un `Cluster` è un database PostgreSQL e un `Pooler` è un connection pooler (PgBouncer) che smista le connessioni verso un database.

**Regola.**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: rule-e2
spec:
  displayName: Rule E2
  category: General
  severity: MEDIUM
  message: "Finding on {{target.metadata.name}}"
  target:
    kind: Pooler
    apiVersion: postgresql.cnpg.io/v1
    plural: poolers
  dependencies:
    - id: cluster
      kind: Cluster
      apiVersion: postgresql.cnpg.io/v1
      plural: clusters
  relationships:
    matchAll:
      - with: cluster
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "metadata.name"
            operator: EQUALS
          - targetPath: "metadata.namespace"
            dependencyPath: "metadata.namespace"
            operator: EQUALS
```

**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-live
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-nobackup
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-typo
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: db-renamed-sb
  namespace: kp-db-a
---
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: db-nobackup
  namespace: kp-db-a
spec:
  cluster:
    name: db-live
---
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: pool-orphan-name
  namespace: kp-db-a
spec:
  cluster:
    name: db-live
---
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: db-renamed-sb
  namespace: kp-db-b
spec:
  cluster:
    name: db-live
---
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: db-typo
  namespace: kp-db-a
spec:
  cluster:
    name: db-live
```

**E2.1** Descrivi in una frase che cosa segnala questa regola.

> 

**E2.2** Quali oggetti vengono segnalati dalla regola?

| Oggetto | Segnalato? |
|---|---|
| Pooler `kp-db-a/db-nobackup` | ☐ sì  ☐ no |
| Pooler `kp-db-a/pool-orphan-name` | ☐ sì  ☐ no |
| Pooler `kp-db-b/db-renamed-sb` | ☐ sì  ☐ no |
| Pooler `kp-db-a/db-typo` | ☐ sì  ☐ no |

**E2.3** Quanto sei sicuro delle tue risposte?  ☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)

**E2.4** Quanti minuti hai impiegato per questo esempio?  ____

### Esempio 3 (sintassi: Kyverno)

**Contesto.** In Argo CD un `ApplicationSet` genera automaticamente delle `Application` (applicazioni da installare nel cluster) a partire da un template e da uno o più generatori.

**Regola.**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: rule-e3
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["argoproj.io"]
        apiVersions: ["v1alpha1"]
        resources: ["applicationsets"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: applications
      expression: globalContext.Get("applications.argoproj.io", "")
  validations:
    - expression: >-
        dyn(variables.applications).exists(d,
          d.metadata.?ownerReferences.orValue([]).exists(o, o.uid == object.metadata.uid))
      messageExpression: "'Finding on ' + object.metadata.name"
```

**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: as-empty-list
  namespace: argocd
  uid: 7c1e0a52-0001-4000-8000-000000000001
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: as-list-1
  namespace: argocd
  uid: 7c1e0a52-0002-4000-8000-000000000002
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: as-cluster-none
  namespace: argocd
  uid: 7c1e0a52-0003-4000-8000-000000000003
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: as-list-1-solo
  namespace: argocd
  ownerReferences:
    - apiVersion: argoproj.io/v1alpha1
      kind: ApplicationSet
      name: as-list-1
      uid: 7c1e0a52-0002-4000-8000-000000000002
```

**E3.1** Descrivi in una frase che cosa segnala questa regola.

> 

**E3.2** Quali oggetti vengono segnalati dalla regola?

| Oggetto | Segnalato? |
|---|---|
| ApplicationSet `argocd/as-empty-list` | ☐ sì  ☐ no |
| ApplicationSet `argocd/as-list-1` | ☐ sì  ☐ no |
| ApplicationSet `argocd/as-cluster-none` | ☐ sì  ☐ no |

**E3.3** Quanto sei sicuro delle tue risposte?  ☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)

**E3.4** Quanti minuti hai impiegato per questo esempio?  ____

### Esempio 4 (sintassi: KubePattern)

**Contesto.** cert-manager emette certificati TLS. Un `ClusterIssuer` è un'autorità di emissione valida in tutto il cluster (la variante limitata a un namespace si chiama `Issuer`); un `Certificate` descrive un certificato da ottenere; una `CertificateRequest` è una singola richiesta di firma.

**Regola.**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: rule-e4
spec:
  displayName: Rule E4
  category: General
  severity: MEDIUM
  message: "Finding on {{target.metadata.name}}"
  target:
    kind: ClusterIssuer
    apiVersion: cert-manager.io/v1
    plural: clusterissuers
  dependencies:
    - id: certificate
      kind: Certificate
      apiVersion: cert-manager.io/v1
      plural: certificates
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
    - id: request
      kind: CertificateRequest
      apiVersion: cert-manager.io/v1
      plural: certificaterequests
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
  relationships:
    matchNone:
      - with: certificate
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
      - with: request
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
```

**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ci-web
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ci-kind-omitted
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ci-manual
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ci-unused
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: web
  namespace: kp-certs
spec:
  issuerRef:
    name: ci-web
    kind: ClusterIssuer
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: kind-omitted
  namespace: kp-certs
spec:
  issuerRef:
    name: ci-kind-omitted
---
apiVersion: cert-manager.io/v1
kind: CertificateRequest
metadata:
  name: manual
  namespace: kp-certs
spec:
  issuerRef:
    name: ci-manual
    kind: ClusterIssuer
    group: cert-manager.io
```

**E4.1** Descrivi in una frase che cosa segnala questa regola.

> 

**E4.2** Quali oggetti vengono segnalati dalla regola?

| Oggetto | Segnalato? |
|---|---|
| ClusterIssuer `ci-web` | ☐ sì  ☐ no |
| ClusterIssuer `ci-kind-omitted` | ☐ sì  ☐ no |
| ClusterIssuer `ci-manual` | ☐ sì  ☐ no |
| ClusterIssuer `ci-unused` | ☐ sì  ☐ no |

**E4.3** Quanto sei sicuro delle tue risposte?  ☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)

**E4.4** Quanti minuti hai impiegato per questo esempio?  ____

### Esempio 5 (sintassi: Kyverno)

**Contesto.** In Crossplane una `Function` è un'estensione installata nel cluster che gira come Pod. Le `Composition`, le `Operation`, le `CronOperation` (operazioni pianificate) e le `WatchOperation` (operazioni attivate da eventi) eseguono una pipeline di passi, e ogni passo invoca una Function.

**Regola.**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: rule-e5
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["pkg.crossplane.io"]
        apiVersions: ["v1"]
        resources: ["functions"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: compositions
      expression: globalContext.Get("compositions.apiextensions.crossplane.io", "")
    - name: operations
      expression: globalContext.Get("operations.ops.crossplane.io", "")
    - name: cronoperations
      expression: globalContext.Get("cronoperations.ops.crossplane.io", "")
    - name: watchoperations
      expression: globalContext.Get("watchoperations.ops.crossplane.io", "")
  validations:
    - expression: >-
        dyn(variables.compositions).exists(d,
          d.spec.?pipeline.orValue([]).exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.operations).exists(d,
          d.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.cronoperations).exists(d,
          d.spec.operationTemplate.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.watchoperations).exists(d,
          d.spec.operationTemplate.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
      messageExpression: "'Finding on ' + object.metadata.name"
```

**Stato del cluster.** Questi sono gli unici oggetti dei tipi coinvolti.

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-auto-ready
---
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-go-templating
---
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-kcl
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: app-default
spec:
  pipeline:
    - step: resources
      functionRef:
        name: function-patch-and-transform
    - step: ready
      functionRef:
        name: function-auto-ready
---
apiVersion: ops.crossplane.io/v1alpha1
kind: CronOperation
metadata:
  name: yearly-report
spec:
  operationTemplate:
    spec:
      pipeline:
        - step: render
          functionRef:
            name: function-go-templating
```

**E5.1** Descrivi in una frase che cosa segnala questa regola.

> 

**E5.2** Quali oggetti vengono segnalati dalla regola?

| Oggetto | Segnalato? |
|---|---|
| Function `function-auto-ready` | ☐ sì  ☐ no |
| Function `function-go-templating` | ☐ sì  ☐ no |
| Function `function-kcl` | ☐ sì  ☐ no |

**E5.3** Quanto sei sicuro delle tue risposte?  ☐ 1 (per niente)  ☐ 2  ☐ 3  ☐ 4  ☐ 5 (del tutto)

**E5.4** Quanti minuti hai impiegato per questo esempio?  ____

## Parte 2: confronto

Ora vedi le stesse cinque regole nelle due sintassi, con i nomi e i messaggi reali. Le `GlobalContextEntry` usate dalle policy Kyverno sono definite a parte, come nel primer.

### Esempio 1

**Contesto.** CloudNativePG gestisce database PostgreSQL su Kubernetes. Un `Cluster` è un database PostgreSQL; uno `ScheduledBackup` pianifica i backup periodici di un database.

**Kyverno**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: cnpg-cluster-without-scheduledbackup
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["postgresql.cnpg.io"]
        apiVersions: ["v1"]
        resources: ["clusters"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: schedules
      expression: globalContext.Get("scheduledbackups.postgresql.cnpg.io", "")
  validations:
    - expression: >-
        dyn(variables.schedules).exists(d, d.metadata.namespace == object.metadata.namespace
          && d.spec.cluster.name == object.metadata.name)
      messageExpression: >-
        'CNPG Cluster ' + object.metadata.namespace + '/' + object.metadata.name +
        ' has no ScheduledBackup in its namespace.'
```

**KubePattern**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: cnpg-cluster-without-scheduledbackup
spec:
  displayName: PostgreSQL Cluster Without Scheduled Backup
  category: Reliability
  severity: HIGH
  message: "CNPG Cluster {{target.metadata.namespace}}/{{target.metadata.name}} has no ScheduledBackup in its namespace."
  reference: "https://cloudnative-pg.io/documentation/current/backup/"
  target:
    kind: Cluster
    apiVersion: postgresql.cnpg.io/v1
    plural: clusters
  dependencies:
    - id: schedule
      kind: ScheduledBackup
      apiVersion: postgresql.cnpg.io/v1
      plural: scheduledbackups
  relationships:
    matchNone:
      - with: schedule
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.cluster.name"
            operator: EQUALS
          - targetPath: "metadata.namespace"
            dependencyPath: "metadata.namespace"
            operator: EQUALS
```

**C1.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C1.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C1.c** Quale delle due versioni preferiresti mantenere nel tempo?  ☐ Kyverno  ☐ KubePattern  ☐ indifferente

**C1.d** Commenti (facoltativo).

> 

### Esempio 2

**Contesto.** In CloudNativePG un `Cluster` è un database PostgreSQL e un `Pooler` è un connection pooler (PgBouncer) che smista le connessioni verso un database.

**Kyverno**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: cnpg-pooler-name-clashes-with-cluster
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["postgresql.cnpg.io"]
        apiVersions: ["v1"]
        resources: ["poolers"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: clusters
      expression: globalContext.Get("clusters.postgresql.cnpg.io", "")
  validations:
    - expression: >-
        !dyn(variables.clusters).exists(d, d.metadata.namespace == object.metadata.namespace
          && d.metadata.name == object.metadata.name)
      messageExpression: >-
        'Pooler ' + object.metadata.namespace + '/' + object.metadata.name +
        ' has the same name as a CNPG Cluster in the same namespace (forbidden by the Pooler API contract).'
```

**KubePattern**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: cnpg-pooler-name-clashes-with-cluster
spec:
  displayName: Pooler Named Like a Cluster
  category: Misconfiguration
  severity: HIGH
  message: "Pooler {{target.metadata.namespace}}/{{target.metadata.name}} has the same name as a CNPG Cluster in the same namespace (forbidden by the Pooler API contract)."
  reference: "https://cloudnative-pg.io/documentation/current/connection_pooling/"
  target:
    kind: Pooler
    apiVersion: postgresql.cnpg.io/v1
    plural: poolers
  dependencies:
    - id: cluster
      kind: Cluster
      apiVersion: postgresql.cnpg.io/v1
      plural: clusters
  relationships:
    matchAll:
      - with: cluster
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "metadata.name"
            operator: EQUALS
          - targetPath: "metadata.namespace"
            dependencyPath: "metadata.namespace"
            operator: EQUALS
```

**C2.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C2.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C2.c** Quale delle due versioni preferiresti mantenere nel tempo?  ☐ Kyverno  ☐ KubePattern  ☐ indifferente

**C2.d** Commenti (facoltativo).

> 

### Esempio 3

**Contesto.** In Argo CD un `ApplicationSet` genera automaticamente delle `Application` (applicazioni da installare nel cluster) a partire da un template e da uno o più generatori.

**Kyverno**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: argocd-applicationset-generates-nothing
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["argoproj.io"]
        apiVersions: ["v1alpha1"]
        resources: ["applicationsets"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: applications
      expression: globalContext.Get("applications.argoproj.io", "")
  validations:
    - expression: >-
        dyn(variables.applications).exists(d,
          d.metadata.?ownerReferences.orValue([]).exists(o, o.uid == object.metadata.uid))
      messageExpression: >-
        'ApplicationSet ' + object.metadata.namespace + '/' + object.metadata.name +
        ' currently owns no Application: its generators match nothing.'
```

**KubePattern**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: argocd-applicationset-generates-nothing
spec:
  displayName: ApplicationSet Generates No Applications
  category: Architecture
  severity: MEDIUM
  message: "ApplicationSet {{target.metadata.namespace}}/{{target.metadata.name}} currently owns no Application: its generators match nothing."
  reference: "https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/"
  target:
    kind: ApplicationSet
    apiVersion: argoproj.io/v1alpha1
    plural: applicationsets
  dependencies:
    - id: application
      kind: Application
      apiVersion: argoproj.io/v1alpha1
      plural: applications
  relationships:
    matchNone:
      - with: application
        type: owns
```

**C3.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C3.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C3.c** Quale delle due versioni preferiresti mantenere nel tempo?  ☐ Kyverno  ☐ KubePattern  ☐ indifferente

**C3.d** Commenti (facoltativo).

> 

### Esempio 4

**Contesto.** cert-manager emette certificati TLS. Un `ClusterIssuer` è un'autorità di emissione valida in tutto il cluster (la variante limitata a un namespace si chiama `Issuer`); un `Certificate` descrive un certificato da ottenere; una `CertificateRequest` è una singola richiesta di firma.

**Kyverno**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: certmanager-clusterissuer-not-used
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["cert-manager.io"]
        apiVersions: ["v1"]
        resources: ["clusterissuers"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: certificates
      expression: globalContext.Get("certificates.cert-manager.io", "")
    - name: requests
      expression: globalContext.Get("certificaterequests.cert-manager.io", "")
  validations:
    - expression: >-
        dyn(variables.certificates).exists(d, d.spec.issuerRef.?kind.orValue('') == 'ClusterIssuer'
          && d.spec.issuerRef.name == object.metadata.name)
        || dyn(variables.requests).exists(d, d.spec.issuerRef.?kind.orValue('') == 'ClusterIssuer'
          && d.spec.issuerRef.name == object.metadata.name)
      messageExpression: >-
        'ClusterIssuer ' + object.metadata.name +
        ' is not referenced by any Certificate or CertificateRequest; it still holds signing material.'
```

**KubePattern**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: certmanager-clusterissuer-not-used
spec:
  displayName: ClusterIssuer Not Used
  category: Security
  severity: LOW
  message: "ClusterIssuer {{target.metadata.name}} is not referenced by any Certificate or CertificateRequest; it still holds signing material."
  reference: "https://cert-manager.io/docs/usage/certificate/"
  target:
    kind: ClusterIssuer
    apiVersion: cert-manager.io/v1
    plural: clusterissuers
  dependencies:
    - id: certificate
      kind: Certificate
      apiVersion: cert-manager.io/v1
      plural: certificates
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
    - id: request
      kind: CertificateRequest
      apiVersion: cert-manager.io/v1
      plural: certificaterequests
      filters:
        matchAll:
          - path: "spec.issuerRef.kind"
            operator: EQUALS
            values: ["ClusterIssuer"]
  relationships:
    matchNone:
      - with: certificate
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
      - with: request
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.issuerRef.name"
            operator: EQUALS
```

**C4.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C4.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C4.c** Quale delle due versioni preferiresti mantenere nel tempo?  ☐ Kyverno  ☐ KubePattern  ☐ indifferente

**C4.d** Commenti (facoltativo).

> 

### Esempio 5

**Contesto.** In Crossplane una `Function` è un'estensione installata nel cluster che gira come Pod. Le `Composition`, le `Operation`, le `CronOperation` (operazioni pianificate) e le `WatchOperation` (operazioni attivate da eventi) eseguono una pipeline di passi, e ogni passo invoca una Function.

**Kyverno**

```yaml
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: crossplane-function-not-used
spec:
  validationActions: [Audit]
  evaluation:
    admission:
      enabled: false
  matchConstraints:
    resourceRules:
      - apiGroups: ["pkg.crossplane.io"]
        apiVersions: ["v1"]
        resources: ["functions"]
        operations: ["CREATE", "UPDATE"]
  variables:
    - name: compositions
      expression: globalContext.Get("compositions.apiextensions.crossplane.io", "")
    - name: operations
      expression: globalContext.Get("operations.ops.crossplane.io", "")
    - name: cronoperations
      expression: globalContext.Get("cronoperations.ops.crossplane.io", "")
    - name: watchoperations
      expression: globalContext.Get("watchoperations.ops.crossplane.io", "")
  validations:
    - expression: >-
        dyn(variables.compositions).exists(d,
          d.spec.?pipeline.orValue([]).exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.operations).exists(d,
          d.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.cronoperations).exists(d,
          d.spec.operationTemplate.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
        || dyn(variables.watchoperations).exists(d,
          d.spec.operationTemplate.spec.pipeline.exists(s, s.functionRef.name == object.metadata.name))
      messageExpression: >-
        'Function ' + object.metadata.name +
        ' runs a Pod but is not referenced by any Composition or Operation pipeline step.'
```

**KubePattern**

```yaml
apiVersion: kubepattern.dev/v1
kind: Pattern
metadata:
  name: crossplane-function-not-used
spec:
  displayName: Crossplane Function Not Used
  category: Cost
  severity: MEDIUM
  message: "Function {{target.metadata.name}} runs a Pod but is not referenced by any Composition or Operation pipeline step."
  reference: "https://docs.crossplane.io/latest/packages/functions/"
  target:
    kind: Function
    apiVersion: pkg.crossplane.io/v1
    plural: functions
  dependencies:
    - id: composition
      kind: Composition
      apiVersion: apiextensions.crossplane.io/v1
      plural: compositions
    - id: operation
      kind: Operation
      apiVersion: ops.crossplane.io/v1alpha1
      plural: operations
    - id: cronoperation
      kind: CronOperation
      apiVersion: ops.crossplane.io/v1alpha1
      plural: cronoperations
    - id: watchoperation
      kind: WatchOperation
      apiVersion: ops.crossplane.io/v1alpha1
      plural: watchoperations
  relationships:
    matchNone:
      - with: composition
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.pipeline[*].functionRef.name"
            operator: EQUALS
      - with: operation
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.pipeline[*].functionRef.name"
            operator: EQUALS
      - with: cronoperation
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.operationTemplate.spec.pipeline[*].functionRef.name"
            operator: EQUALS
      - with: watchoperation
        type: custom
        criteria:
          - targetPath: "metadata.name"
            dependencyPath: "spec.operationTemplate.spec.pipeline[*].functionRef.name"
            operator: EQUALS
```

**C5.a** Quanto è facile capire la regola? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C5.b** Quanto sarebbe facile modificarla, per esempio per far contare come riferimento anche un altro tipo di risorsa? (1 = molto difficile, 5 = molto facile)

- Kyverno:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5
- KubePattern:  ☐ 1  ☐ 2  ☐ 3  ☐ 4  ☐ 5

**C5.c** Quale delle due versioni preferiresti mantenere nel tempo?  ☐ Kyverno  ☐ KubePattern  ☐ indifferente

**C5.d** Commenti (facoltativo).

> 

## Parte 3: giudizio complessivo

**G1** In generale, quale sintassi trovi più leggibile?  ☐ Kyverno  ☐ KubePattern  ☐ nessuna differenza

**G2** Quale sceglieresti per scrivere regole di questo tipo nel tuo lavoro, e perché?  ☐ Kyverno  ☐ KubePattern  ☐ nessuna preferenza

> 

**G3** Commenti liberi.

> 
