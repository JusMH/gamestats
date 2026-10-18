# Auditoría técnica y documental final

Corte: **18 de octubre de 2026**. Repositorio JusMH/gamestats, rama de revisión
docs/final-academic-audit, base main/origin/main
7782fa9b4c88a73317a3b0b1eb51d0fbacca5d15. La revisión solo cambia Markdown;
no modifica ejecutables, contratos JSON, perfiles, fixtures, workflows, Dockerfiles,
tag o adjuntos publicados. No crea commits, pushes, PRs, merges ni Releases.

## Fuentes y alcance de la comprobación

Se inspeccionaron los módulos Go de eventos/partidas/HTTP/análisis y sus pruebas;
Model, Decoding, Engine, TemporalWindow, TicTacToeLine, HttpService y Main de Scala;
productores, cliente, expectativas, negativos y coordinación del simulador;
Dockerfiles, Compose, CI, publicación y verificadores. La documentación se
contrasta con código y pruebas, no se usa como prueba de sí misma.

La referencia académica es **Proyecto1 - GameStats_Enunciado.docx**, proporcionado
por el usuario fuera del repositorio. SHA-256 del original leído, sin modificar:
665622e370ff76a1018c19bf228c3498012b1d8892a6e99a8e461108a6e4d4f0.
No se añade el archivo privado al repositorio ni se reconstruye una rúbrica.
Las referencias de sección siguientes corresponden a ese enunciado; no asignan
puntos ni sustituyen la aceptación docente. La entrega administrativa, composición
del equipo y presentación en TecDigital no quedan certificadas por pruebas de software.

## Matriz del enunciado contra evidencia

| Requisito original | Implementación y evidencia real | Límite o estado |
| --- | --- | --- |
| §1–5: plataforma genérica y decisiones de eventos/estadísticas | Sobre v1 y perfiles de tres juegos; [eventos](events.md), [juegos](games.md), [plantillas](analysis-templates.md). | No es una consola ni implementa videojuegos. |
| §6.1: entrada externa y validación básica Go | POST /events, json.RawMessage, validación estructural y límite de cuerpo; internal/events y pruebas HTTP. | No valida reglas particulares del videojuego. |
| §6.1: flujo, clasificación, estado y partidas concurrentes | Clave (gameId, matchId), inicio/cierre, duplicados, RWMutex de registro/partida y snapshots consistentes. | Memoria local; no hay cola asíncrona propia, persistencia o exactamente una vez. |
| §6.1 y §7: envío a Scala y módulos independientes | Cliente HTTP Go → POST /analyze, GET /results bajo demanda; [contrato](integration-api.md), integración real en CI. | Go no calcula estadísticas; upstream fallido produce 502/504 según el caso. |
| §6.2: transformación funcional de colecciones | Motor puro con filter/map/groupBy/fold/flatMap y extracción tipada donde corresponde; pruebas sin servidor. | El enunciado permite operaciones cuando sean apropiadas, no exige introducirlas artificialmente. |
| §6.2: estadísticas y resultados estructurados | count/sum/avg decimal, duración, composición, modelos Metric/Pattern y procedencia/basis. | Sin min/max ni motor universal. |
| §8: fuente simulada, varios juegos/jugadores/partidas/momentos | Seis escenarios propios deterministas, alice/bob, victorias/empate, Snake y carreras; barreras por fase y HTTP. | No simula física ni es benchmark de carga. |
| §9: múltiples eventos y estadísticas por jugador y partida | Movimientos, manzanas, puntuación, vueltas y duración; 13 fixtures y expectativas independientes del simulador. | Se analiza una partida por snapshot, no agregados globales entre partidas. |
| §9: estadística que combina distintos tipos de eventos | duration/v1 relaciona MATCH_STARTED con MATCH_FINISHED o PLAYER_DIED. | Fin ausente no inventa duración ni timestamp actual. |
| §9: secuencia/combinación de acontecimientos | temporal-window/v1 agrupa tres manzanas en diez segundos; Gato reconstruye tablero y primera línea. | Greedy sin solapamiento; Gato valida solo el alcance acordado. |
| §9: consulta/presentación de resultados | GET /matches y GET /results; logs y resumen del simulador; resultados abiertos/finales. | Consulta HTTP/polling, sin push. |
| §10: Git y construcción reproducible | Historial remoto, PRs fusionados, commit/tag identificados; versiones de módulos y builders fijadas. | Esta auditoría no crea historial nuevo ni modifica v1.0.0. No se promete igualdad binaria universal de builds. |
| §10: Docker y configuración integrada | Tres imágenes multietapa, DNS interno, healthchecks y simulador como tarea; Compose de desarrollo y paquete por digest. | Linux/amd64 probado; ARM64 no acreditado. |
| §10: pipeline CI/CD mínimo | CI 39104918511 success: build, pruebas Go/race y Scala, fixtures, empaquetado e integración Docker. Workflow separado publica artefactos. | La CI pasa; Finalize del workflow de publicación falló. No se oculta ese fallo. |
| §10: deployment funcional en entorno elegido | ZIP evaluado sin fuentes, imágenes anónimas, Go/Scala healthy y dos demos mediante Compose local. | Entorno elegido por el equipo: Docker Desktop/motor compatible, Linux/amd64. El enunciado no exige nube. |
| §11: fuente, especificaciones, configuraciones y manuales | Repositorio, contratos, perfiles, Dockerfiles, workflows, ZIP, instalación/guía y [índice completo](README.md). | La entrega administrativa final al profesor no se deduce de una Release. |
| §12–13: Go/Scala, extensibilidad y justificación de paradigmas | Go imperativo/sincronizado para ingestión; Scala inmutable/funcional para análisis; [arquitectura](architecture.md), [nuevo juego](integrating-new-games.md). | Nueva configuración no exige cambiar Go; nueva especialización sí necesita Scala y pruebas. |

## Estado de Release, CI y publicación: evidencias separadas

- [Release v1.0.0](https://github.com/JusMH/gamestats/releases/tag/v1.0.0):
  publicada, no draft ni prerelease; la API informa immutable=false. Tag remoto
  y manifiesto fijan el commit 7782fa9b4c88a73317a3b0b1eb51d0fbacca5d15.
- [CI 39104918511](https://github.com/JusMH/gamestats/actions/runs/39104918511):
  push/main de ese mismo commit, success en sus seis jobs; race detector pasó
  en go-ingestion y simulator. No se confunde con la publicación.
- [Publicación 39105352591, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2):
  approval/images/evaluation success, finalize failure. El log termina con
  `Draft creation not confirmed; retry failed job` después de crear un borrador
  observado bajo un tag provisional. No se declara reparado ese comportamiento.
- La recuperación manual publicada por el mantenedor queda respaldada por los
  tres adjuntos oficiales y sus fechas/uploader posteriores al fallo. El ZIP
  publicado y el artefacto de evaluación tienen el mismo SHA-256
  ec39727548d155923f6631bfb9ee37c64cda0ae80d3dc72d4321368ceefd9ad6.
  No se inventa la secuencia exacta de clics usada para la recuperación.
- Se descargaron anónimamente ZIP, lock y checksum, y se comprobaron sus hashes,
  su estructura y el lock embebido. Los digests efectivos y hashes de adjuntos
  están en [distribución](academic-distribution.md), como fuente única detallada.
- Los tres manifiestos GHCR se recuperaron nuevamente con token anónimo de pull
  y hash coincidente. No se imprimen ni guardan tokens. Esto es una comprobación
  ligera actual; no equivale a volver a descargar todas las capas y ejecutar Docker.

El artefacto academic-evaluation-39105352591-2 confirmó status=passed,
anonymousPulls=true y distributionIntegrationExecuted=true. Primera y nueva
execución: cada una 6 partidas, 50 eventos y 60 consultas (48 abiertas, 12 cerradas).
Negativos: 3; IDs reutilizados: 6 rechazos 409; resourcesReleased=true. El pico
observado fue 6 solicitudes; el criterio exige concurrencia, no ese valor exacto.
El intento 2 reutilizó release-images-39105352591-1 sin reconstruir ni repush.
La retención de artefactos es 14 días: no se promete su disponibilidad indefinida.

La Release manual no tiene el marcador de propietario run-id/SHA esperado por
Finalize. No se debe reejecutar ciegamente ese job sobre v1.0.0 ni mover su tag.
Corregir el finalizador y verificar otra publicación requiere una tarea separada.
El comentario «imágenes aún NO publicadas» dentro del Compose del ZIP es una
errata cosmética: las referencias reales están publicadas y fijadas por digest.
Se documenta sin sustituir el adjunto ya publicado.

## Contradicciones corregidas

| Antes | Después y criterio |
| --- | --- |
| Arquitectura y contratos presentaban Scala/simulador como futuros. | Se describe la implementación real y se conservan esquemas JSON. |
| README y hoja de ruta decían que no existía entrega publicada. | Se enlaza v1.0.0 y se documentan hashes, evaluación y recuperación. |
| «No hay CI/deployment» en manuales de componentes. | CI comprobada y deployment local Compose, separados de nube no implementada. |
| Diseño del finalizador descrito como garantía suficiente. | Se distingue intención, tests sintéticos y fallo real remoto. |
| Fixtures descritos como futuros y servicio inexistente. | Son oráculos comparados con 13 respuestas completas de Scala; check.py sigue siendo solo un verificador acotado. |
| Race pendiente en Windows parecía una limitación global. | Se conserva la nota histórica y se enlazan ambos race checks Linux aprobados. |
| «Go 1.22 o posterior» sin distinguir evidencias. | go.mod declara 1.22; CI/Docker prueban 1.26.5. No se afirma incompatibilidad con 1.22. |
| JSON ambiguo en data se describía como dominio invalid de Scala. | El parser rechaza claves duplicadas con HTTP 400; Go presenta 502 al consultar. |
| Una versión exacta de Java podía confundirse entre entornos. | CI selecciona Temurin 21 disponible; Docker fija patch/digest. |
| Propuestas históricas parecían tareas aún pendientes. | Banners de 4A/4B y secciones históricas enlazan estado vigente. |

## Integridad, inventario y duplicados

El [índice](README.md) enumera y clasifica los **22 Markdown**: 19 revisados y
3 nuevos (índice, guía de integración y este informe). No se elimina ninguno.
Los contratos son normativos; manuales operan el sistema; 4A/4B conservan historia.
Los quickstarts repetidos tienen públicos distintos y enlazan fuentes de detalle;
no hay redundancia completa que justifique borrar decisiones o evidencia.

Las correcciones de distribution/*.md son plantillas para un futuro paquete,
no cambios al ZIP descargado por el profesor. La única modificación dentro de
fixtures es su catálogo Markdown: ningún JSON, expected, manifest o check.py cambia.
La auditoría verifica además que los ejemplos JSON existentes no se reescriben.

## Verificaciones ejecutadas durante esta auditoría

| Comprobación local | Resultado |
| --- | --- |
| Preparación Git | Árbol inicial limpio; fetch sin tags, main actualizado por avance de referencia y rama nueva desde main, sin merge. |
| Verificador documental existente | PASS: 30 JSON, 13 pares de fixtures y 9 mutaciones inválidas. |
| unittest infraestructura | 16 pruebas descubiertas y ejecutadas, todas OK. |
| unittest distribución | 24 pruebas descubiertas y ejecutadas, todas OK; no subsanan el fallo remoto. |
| Compose desarrollo y evaluación | Ambas configuraciones válidas mediante config --quiet. No se reconstruyeron imágenes. |
| Runtime Scala local existente | check-stage.py: app.jar + 5 dependencias, health, 13 respuestas completas y salida 1 con perfiles vacíos. |
| Ejemplo nuevo score.player | Perfil copiado a directorio ignorado junto con los tres aprobados; 8 comprobaciones HTTP reales: open/closed, repetición, texto/null/negativo invalid y juego sin perfil. |
| Integridad documental final | 22 Markdown, 150 enlaces relativos y anclas válidos, 18 bloques JSON válidos; ejemplos JSON de los 19 documentos existentes intactos; sintaxis de los 8 Python versionados válida. |
| Integridad de Git | git diff --check sin errores; 19 Markdown modificados y 3 nuevos, sin cambios funcionales ni temporales sin seguimiento. Tag remoto v1.0.0 confirmado sin cambios. |

check-stage usa el artefacto ya preparado, no es una nueva compilación de Scala
ni una nueva ejecución Docker. Durante esta auditoría no se repiten las suites
Go/race, MUnit 143 ni la integración completa de contenedores: sus éxitos se
recuperan de CI/evaluación reales del commit identificado. No se presentan como
pruebas recién ejecutadas. No se prueba un dispositivo físico del profesor.

## Límites, riesgos y decisiones pendientes

La [arquitectura](architecture.md#límites-comprobados-en-código) centraliza límites
por componente. Go conserva sin límite global los eventos en memoria; Scala
limita snapshot a 10 000 eventos/8 MiB, 1 000 jugadores y 128 perfiles. Una
partida aceptada por Go puede resultar demasiado grande para analizar. El puente
espera como máximo 5 segundos; no se promete throughput sin benchmarks.

Los perfiles se cargan al inicio. No hay recarga dinámica, persistencia,
recuperación, caché, push ni estadísticas históricas globales. Gato no comprueba
alternancia, X inicial ni detención tras victoria; no es un verificador universal
de legalidad. Los patrones de partidas abiertas pueden cambiar por eventos tardíos.

Decisiones humanas pendientes: revisar la reparación de Finalize y autorizar una
futura versión; confirmar condiciones administrativas de entrega/defensa con el
docente. Prolog es una posible integración **FUTURA**, sin implementación. ARM64,
offline completo, nube y prestaciones masivas no se declaran compatibles/probadas.
No se propone infraestructura adicional para cumplir supuestos no contenidos en
el enunciado.

## Recomendación de versión documental

Revisar estos cambios como documentación nueva sobre main. Conservar v1.0.0,
los adjuntos y los digests actuales; no hacer una sustitución retroactiva. Si se
desea un nuevo ZIP con manuales corregidos, decidir una nueva versión después
de reparar Finalize y repetir su verificación remota completa. Esta auditoría
no crea esa versión ni garantiza la futura publicación automática.
