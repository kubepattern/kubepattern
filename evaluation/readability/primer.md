# Primer: due modi di scrivere la stessa regola

Nel sondaggio vedrai regole che controllano un cluster Kubernetes e segnalano gli oggetti che hanno un problema. Ogni regola è scritta in una di due sintassi: una **Pattern** di KubePattern oppure una **ValidatingPolicy** di Kyverno. In entrambi i casi la regola viene valutata periodicamente sugli oggetti che esistono già nel cluster, e non blocca la creazione di nuovi oggetti.

Leggi questa pagina prima di iniziare. Puoi riaprirla in qualsiasi momento durante il sondaggio.

## Nozioni comuni
- Ogni oggetto ha un tipo (`kind`, per esempio `Cluster`) e un nome (`metadata.name`). Se il tipo è namespaced, ha anche un namespace (`metadata.namespace`). Alcuni tipi, come `ClusterIssuer` o `Function`, non hanno namespace.
- `metadata.uid` è l'identificativo unico di un oggetto.
- `metadata.ownerReferences` elenca i proprietari di un oggetto, cioè chi l'ha creato e ne governa il ciclo di vita. Ogni voce riporta `kind`, `name` e `uid` del proprietario.
- Un percorso come `spec.cluster.name` indica un campo annidato: il campo `name` dentro `cluster`, dentro `spec`.

## KubePattern (Pattern)
Una Pattern descrive il **difetto**: ogni oggetto che soddisfa la condizione viene segnalato.

```yaml
spec:
  displayName: ...   # descrizione della segnalazione
  category: ...      # (anche severity e message)
  target:            # il tipo di oggetto controllato: ogni oggetto di questo tipo è un candidato
  dependencies:      # gli altri tipi da consultare, ciascuno con un id
  relationships:     # le relazioni attese tra il target e le dipendenze
```

- `target` e ogni dipendenza indicano un tipo con `kind`, `apiVersion` e `plural`.
- Target e dipendenze possono avere dei `filters`, cioè condizioni su un singolo oggetto (`path`, `operator`, `values`). Con `matchAll`, un oggetto viene considerato solo se tutte le condizioni valgono. Se il campo manca, la condizione non vale.
- Una dipendenza comprende tutti gli oggetti del suo tipo, in tutto il cluster. Il namespace conta solo se un criterio lo confronta esplicitamente.
- Una relazione (`with: <id>`) vale per un target se **almeno un** oggetto della dipendenza la soddisfa. Ci sono due tipi di relazione:
  - `custom`: vale se tutti i `criteria` valgono per lo stesso oggetto. Un criterio confronta un campo del target (`targetPath`) con un campo della dipendenza (`dependencyPath`); `EQUALS` vale se i due campi hanno almeno un valore in comune. `[*]` indica tutti gli elementi di una lista: `spec.pipeline[*].functionRef.name` sono i nomi presenti in tutti i passi della pipeline.
  - `owns`: vale se il target è proprietario della dipendenza, direttamente o attraverso una catena di `ownerReferences`.
- Le relazioni sono raggruppate:
  - con `matchNone` il target viene segnalato se **nessuna** relazione del gruppo vale;
  - con `matchAll` viene segnalato se **tutte** valgono;
  - con `matchAny` viene segnalato se almeno una vale.

## Kyverno (ValidatingPolicy)
Una policy descrive la **conformità**: l'espressione deve essere vera per gli oggetti corretti. Se è falsa, l'oggetto viene segnalato.

```yaml
spec:
  validationActions: [Audit]   # segnala senza bloccare
  evaluation:
    admission:
      enabled: false           # nessun controllo alla creazione, solo quello periodico
  matchConstraints:            # il tipo di oggetto controllato (gruppo, versione, risorsa);
                               # "operations" è richiesto dalla sintassi
  variables:                   # valori calcolati una volta e poi riusati
  validations:
    - expression: ...          # condizione CEL: vera = conforme, falsa = segnalato
      messageExpression: ...   # messaggio della segnalazione
```

- `object` è l'oggetto controllato, e `variables.<nome>` è una variabile definita in `variables`.
- `globalContext.Get("certificates.cert-manager.io", "")` restituisce la lista di tutti gli oggetti di quel tipo presenti nel cluster. Ogni tipo letto in questo modo richiede una `GlobalContextEntry`, definita una volta sola e condivisa da tutte le policy:
  ```yaml
  apiVersion: kyverno.io/v2
  kind: GlobalContextEntry
  metadata:
    name: certificates.cert-manager.io
  spec:
    kubernetesResource:
      group: cert-manager.io
      version: v1
      resource: certificates
  ```
- Il linguaggio delle espressioni è CEL:
  - `lista.exists(d, condizione)` è vera se **almeno un** elemento `d` della lista soddisfa la condizione;
  - `&&` significa "e", `||` significa "o", `!` significa "non", `==` significa "uguale";
  - `a.?b.orValue(x)` vale `a.b` se il campo esiste, altrimenti vale `x`;
  - `dyn(...)` è una conversione tecnica di tipo e puoi ignorarla;
  - `'testo' + valore` concatena delle stringhe.
