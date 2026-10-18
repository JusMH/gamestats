# Publicación GHCR y distribución académica

## Estado real de v1.0.0

La [Release v1.0.0](https://github.com/JusMH/gamestats/releases/tag/v1.0.0)
está publicada desde 2026-10-18 07:17:18 UTC. El tag y el lock de imágenes
apuntan a **7782fa9b4c88a73317a3b0b1eb51d0fbacca5d15**, integrado en main
mediante PR #12. Su [CI 39104918511](https://github.com/JusMH/gamestats/actions/runs/39104918511)
terminó success con los seis jobs, incluidos ambos detectores de carreras Go.

La [publicación 39105352591, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2)
terminó **failure**: approval, images y evaluation pasaron, pero finalize falló.
No se presenta el workflow completo como aprobado. La recuperación manual
publicó los tres adjuntos; el ZIP publicado coincide byte a byte con el ZIP
que pasó evaluación anónima. La auditoría de documentación no vuelve a publicar
ni modifica ese tag, las imágenes o los adjuntos.

| Adjunto oficial | Tamaño | SHA-256 |
| --- | --- | --- |
| GameStats-Entrega-v1.0.0.zip | 5 140 bytes | ec39727548d155923f6631bfb9ee37c64cda0ae80d3dc72d4321368ceefd9ad6 |
| images.json | 734 bytes | 3e5c369cf2c9d64e934968771c00e44a7e828c55c4dc083ebea09382fa49e193 |
| SHA256SUMS.txt | 95 bytes | ede68e30788e7761c3418c3bbfd82dbde080be24feb681171f9ea1eddc442936 |

SHA256SUMS.txt comprueba el ZIP; IMAGENES.json dentro del ZIP coincide con
images.json de la Release. Los archivos «Source code» automáticos no son la
entrega para el profesor. La API informa immutable=false para esta Release;
no se afirma que se haya habilitado inmutabilidad en el repositorio.

## Imágenes publicadas y referencias inmutables

Plataforma construida y evaluada: **linux/amd64**. No se declara ARM64 probado.

| Imagen (todas con tag v1.0.0) | Digest publicado |
| --- | --- |
| ghcr.io/jusmh/gamestats-go-ingestion | sha256:426a9c808b0db50bb77b3174f865b70770743cb17d300219e64037b8ed888d6c |
| ghcr.io/jusmh/gamestats-scala-analysis | sha256:0afc0d48f6949b5583984b0d57a6aaa49f612449d2b27d6d56d15379078c5c3f |
| ghcr.io/jusmh/gamestats-simulator | sha256:0a73a2324d09a016a798082f0e1aae66d95f496698859094324710d3a7d72b2a |

El Compose del ZIP usa image: PAQUETE:v1.0.0@sha256:DIGEST_REAL, sin build,
fuentes ni mounts locales. El digest fija contenido; el tag facilita lectura.
No se usa latest como referencia de entrega. Se reutilizan los Dockerfiles
aprobados: Go 1.26.5, JDK/JRE 21, Scala 3.3.7, sbt 1.11.7; bases y digests en
[Docker](docker.md). Se mantienen usuarios sin privilegios y perfiles incluidos.

Durante la auditoría se descargaron los tres manifiestos GHCR sin credenciales
personales y se verificaron sus SHA-256 contra el lock. Eso confirma acceso
anónimo actual a los manifiestos; la descarga completa e integración se acredita
por la evaluación remota, no por ese control ligero. Un repositorio público no
hace públicos automáticamente sus paquetes.

## Instalación y deployment del evaluador

Descargar el ZIP oficial, extraerlo y abrir una terminal en su carpeta:

```sh
docker compose up -d --wait
docker compose ps
docker compose run --rm simulator --run-id evaluacion-01
docker compose run --rm simulator --mode negative --run-id negativos-01
docker compose down
```

Requiere Docker Desktop o motor compatible con Compose y contenedores Linux/amd64.
No requiere Git, Python, Go, Java, Scala ni sbt. La primera descarga requiere
Internet: no es una distribución completamente offline. Go publica
127.0.0.1:8080; Scala es interno. Consultar http://127.0.0.1:8080/health.
Para cambiar el puerto en PowerShell, antes del arranque: `$env:GO_PORT="8081"`.
Para otra demo sin reiniciar Go, elegir otro run-id. Reiniciar Go pierde las
partidas en memoria; no hay persistencia ni histórico.

Esta es la modalidad de **deployment local** elegida para el proyecto. GHCR y
GitHub Release distribuyen artefactos, no alojan Go/Scala como servicio remoto.
No se implementó deployment en nube. Los manuales fuente están en
[INSTALACION](../distribution/INSTALACION.md) y
[GUIA-DE-EVALUACION](../distribution/GUIA-DE-EVALUACION.md); son plantillas para
futuros paquetes, y sus correcciones no alteran lo ya publicado.

El ZIP contiene exactamente:

```text
GameStats-Entrega-v1.0.0/
  compose.yaml
  INSTALACION.md
  GUIA-DE-EVALUACION.md
  IMAGENES.json
```

**Errata documental del ZIP v1.0.0:** el comentario inicial de compose.yaml dice
«imágenes aún NO publicadas». Es un comentario obsoleto de la plantilla; las
referencias efectivas sí contienen los digests publicados y evaluados. No afecta
la ejecución. Se conserva intacto el adjunto; la errata se corrige aquí, sin
reescribir ni reemplazar v1.0.0.

## Evidencia de evaluación del paquete

El artefacto academic-evaluation-39105352591-2 registró:

| Comprobación | Resultado observado |
| --- | --- |
| Estado | status=passed, anonymousPulls=true, distributionIntegrationExecuted=true |
| Primera demo | 6 partidas, 50 eventos, 60 consultas: 48 abiertas y 12 cerradas |
| Nueva demo con otro ID | 6 partidas, 50 eventos, 60 consultas: 48 abiertas y 12 cerradas |
| Negativos | 3 comprobaciones aprobadas |
| IDs repetidos | 6 partidas rechazadas por HTTP 409 |
| Concurrencia | Pico observado de 6 solicitudes; el criterio no exige exactamente 6 |
| Limpieza | resourcesReleased=true |

El verificador extrajo el ZIP fuera de las fuentes, usó DOCKER_CONFIG temporal
vacío, descargó imágenes y ejecutó los servicios reales. No utilizó mocks ni
build del repositorio para acreditar la instalación del profesor. Las imágenes
compartidas descargadas no se borran; sí sus contenedores/red de prueba.

Para repetirlo como mantenedor (Python es requisito de esta herramienta,
no del evaluador):

```sh
python distribution/verify_package.py --zip GameStats-Entrega-v1.0.0.zip --evidence docker/evidence/evaluacion-publicada-01
```

Los artefactos de Actions tienen retención de 14 días. La Release, los checksums,
los digests y los resultados resumidos arriba permiten identificar la entrega;
no debe dependerse indefinidamente de un enlace temporal a un artefacto.

## Workflow previsto y fallo conocido de Finalize

[academic-release.yml](../.github/workflows/academic-release.yml) es independiente
de CI y se inicia por workflow_dispatch desde main. La aprobación humana indica
versión, SHA completo revisado y confirmación exacta `PUBLICAR VERSION`. La CI
exitosa debe corresponder a push/main del mismo SHA y a sus seis jobs esperados.
No se publica desde PR ni se utiliza un PAT.

| Job | Permisos | Función |
| --- | --- | --- |
| approval | contents: read, actions: read | Verificar aprobación manual, main, SHA, CI exacta y ausencia de versión previa. |
| images | contents: read, packages: write | Construir tres imágenes, rechazar tags existentes, publicar y registrar digests/OCI. |
| evaluation | contents: read, actions: read | Generar ZIP candidato y evaluar instalación anónima sin fuentes. |
| finalize | contents: write, actions: read | Pretende crear borrador, verificar los tres adjuntos y publicar al final. Tiene un fallo remoto conocido. |

CI mantiene permisos de lectura. Las acciones están fijadas por SHA; las
versiones y paquetes existentes no se sobrescriben silenciosamente. No hay una
transacción atómica entre tres pushes: un push parcial exige revisión y puede
requerir otra versión. No se cambia visibilidad ni configuración global automáticamente.

Crear borrador → adjuntar/verificar → publicar es el orden compatible con
Releases inmutables. La implementación no logró completar ese orden en la primera
publicación: el log de Finalize muestra una URL de borrador con tag provisional
untagged-2c317717bcc75929ce07 y después:

```text
ValueError: Draft creation not confirmed; retry failed job
```

El fallo ocurre al confirmar el borrador creado (release.py, función finalize).
La discrepancia entre el borrador observado y la búsqueda posterior requiere
investigación; no se declara una causa interna resuelta. Las pruebas sintéticas
locales no detectaron este comportamiento remoto. La recuperación manual posterior
está respaldada por los adjuntos y su coincidencia con el artefacto evaluado,
no por un éxito del job finalize.

El cuerpo de la Release manual es «GameStats academic distribution», sin el
marcador run-id/SHA que exige el finalizador para reconocer una Release propia.
**No reejecutar a ciegas Finalize sobre v1.0.0**: esa recuperación manual no equivale
a un estado automáticamente reanudable. No mover el tag ni reemplazar adjuntos.

## Reejecución y próximas publicaciones

Tras convertir paquetes privados en públicos con autorización administrativa,
**Re-run failed jobs** conserva artefactos del productor exitoso. En el intento 2
se recuperó release-images-39105352591-1; images no reconstruyó ni volvió a publicar.
No usar Re-run all jobs o un nuevo dispatch para reconstruir una versión ya existente.

Antes de otra versión se debe reparar Finalize en una tarea separada, probar la
reconciliación del borrador/adjuntos y completar una ejecución remota exitosa.
Luego, con revisión humana: main aprobado por CI → dispatch de nueva versión y
SHA → GHCR → visibilidad pública autorizada si hace falta → evaluación anónima
→ borrador completo → publicación final. No crear tag/Release por adelantado.
La versión exacta futura necesita decisión humana; no se programa aquí una publicación.

Ante artefactos expirados, pushes parciales o una Release ajena, detenerse y revisar;
no reconstruir/sobrescribir v1.0.0. No se garantiza reanudación automática de todas
las fallas. El flujo no modifica reglas de protección ni despliega en nube.

## Comprobaciones locales reproducibles

```sh
python -m unittest discover -s distribution -p "test_*.py" -v
python -m unittest discover -s docker -p "test_*.py" -v
python docs/fixtures/analysis-v1/check.py
docker compose -f distribution/compose.yaml --profile tools config --quiet
```

Las pruebas de control/ZIP no sustituyen publicación real ni integración GHCR.
[La auditoría](academic-audit.md) separa lo ejecutado durante esta revisión de
los resultados recuperados de CI/Release. ARM64, offline completo y alojamiento
remoto no se ofrecen como verificados.
