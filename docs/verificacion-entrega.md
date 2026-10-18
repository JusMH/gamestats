# Verificación de entrega de GameStats

Revisión del 18 de octubre de 2026. Fuentes de avanceFran sobre
`2d32836a8da097de9a43c431d79a2bcea675c5cf` (main/v1.0.1), con ajustes locales
posteriores. Este informe describe la revisión técnica; no certifica la entrega
administrativa ni sustituye la aceptación del docente.

Se volvió a leer completo **Proyecto1 - GameStats_Enunciado.docx**, proporcionado
fuera del repositorio. Su SHA-256 es
`64aac758d62406afdbb53c30015b520aaaadae5cc64e213ca7017905f0ed8ab6`.
Las ocho filas siguientes corresponden exactamente a sus entregables de §11.
El documento original no se modifica ni se incorpora a las fuentes.

## Entregables, uno por uno

| Entregable del enunciado | Qué está hecho y dónde comprobarlo | Qué queda |
| --- | --- | --- |
| 1. Código fuente completo Go y Scala | [Go](../go-ingestion/), [Scala](../scala-analysis/), [simulador externo](../simulator/). Servicios independientes, ingestión concurrente y motor funcional implementados. | Subir los ajustes locales mediante PR; todavía no forman parte de main ni de v1.0.1. |
| 2. Especificación del formato de eventos | [Sobre v1](events.md), [intercambio HTTP](integration-api.md), ejemplos y 13 pares de [fixtures](fixtures/analysis-v1/README.md). IDs, fechas, jugador opcional, data, ciclo de vida, duplicados y errores documentados. | No se identificó una especificación obligatoria ausente. Mantener estos contratos junto con el código entregado. |
| 3. Juegos considerados y justificación de eventos | [Juegos](games.md) describe Gato, Snake y carreras; [perfiles](examples/profiles/) conectan eventos con métricas y [extensión](integrating-new-games.md) explica nuevos juegos. | Preparar la explicación oral de por qué esos eventos bastan para calcular las métricas. El enunciado no impone una cantidad fija de juegos. |
| 4. Descripción de estadísticas y reglas | [Métricas y patrones](metrics-and-patterns.md), [plantillas](analysis-templates.md), contratos por juego y perfiles ejecutables. Conteos/suma/promedio, duración, ventanas y línea ganadora. | No se identificó una funcionalidad mínima ausente. Explicar en defensa el alcance acotado de cada regla. |
| 5. Dockerfiles y configuración | [Go](../go-ingestion/Dockerfile), [Scala](../scala-analysis/Dockerfile), [simulador](../simulator/Dockerfile), [Compose](../compose.yaml), imágenes fijadas y [manual](docker.md). Construcción e integración real comprobadas. | Para construir en este equipo, usar una ruta completa sin tildes; no basta con cambiar el nombre de la carpeta final. |
| 6. Pipeline CI/CD | [CI](../.github/workflows/ci.yml): compilación, pruebas/race, contratos, empaquetado e integración Docker. [Publicación](../.github/workflows/academic-release.yml): imágenes, evaluación y Release manual. v1.0.1 tiene ambas ejecuciones aprobadas. | El nuevo PR debe aprobar CI para su propio commit. La aprobación de v1.0.1 no prueba automáticamente los nuevos cambios. |
| 7. Instalación, ejecución y deployment | [README](../README.md), [guía CMD](windows-cmd.md), [Docker](docker.md), [distribución](academic-distribution.md) y manuales dentro del ZIP v1.0.1. Entorno elegido: Compose local Linux/amd64. | Definir qué versión se presentará al docente: las fuentes nuevas o el paquete publicado. El ZIP v1.0.1 conserva el código anterior. No se ha autorizado otra publicación. |
| 8. Demostración funcional | Simulador verificado con seis partidas de tres juegos, varios jugadores, consultas abiertas/finales, negativos y concurrencia. [Procedimiento CMD](windows-cmd.md) y verificador Docker reproducibles. | Hacer el ensayo de la presentación y la demostración frente al docente; las pruebas no sustituyen esa actividad. Confirmar si pide video u otro archivo adicional. |

## Requisitos funcionales y académicos contrastados

| Sección | Evidencia | Resultado técnico |
| --- | --- | --- |
| §1–5: eventos genéricos y diseño propio | Sobre común, data extensible, perfiles separados y tres juegos de naturaleza diferente. | Implementado; añadir un juego con capacidades existentes no exige cambiar Go. |
| §6.1: Go recibe, valida, administra y agrupa | API externa, registro por juego/partida, inicio/cierre/duplicados, copias defensivas y mutex por partida. | Implementado; Go no calcula estadísticas. |
| §6.1: concurrencia | net/http, bloqueos separados, pruebas con solicitudes y escrituras simultáneas; simulador con goroutines y barreras de canales. | Implementado y probado. No se exige introducir canales artificiales en el servidor. |
| §6.2: Scala funcional | Colecciones inmutables, filter/map/groupBy/fold/flatMap, composición por registro, pruebas del motor puro. | Implementado; el enunciado pide operaciones cuando sean apropiadas. |
| §7: componentes independientes | Simulador → HTTP Go → HTTP Scala; contratos documentados. | Comprobado con servicios reales. |
| §8: datos variados y juegos en paralelo | Gato con victoria/empate, Snake con/sin ráfaga, dos carreras; alice/bob; fases y tiempos distintos. | Seis partidas, 50 eventos y 60 consultas por demostración. |
| §9: estadísticas por jugador | Movimientos, manzanas, puntuación, vueltas y tiempos según perfil. | Implementado. |
| §9: estadísticas por partida | Conteos y duración de partida. | Implementado. |
| §9: relación de distintos tipos de eventos | duration/v1 relaciona MATCH_STARTED con MATCH_FINISHED o PLAYER_DIED. | Cumple esta clase de requisito; no depende solamente de contar un tipo. |
| §9: secuencia o combinación | Tres manzanas en diez segundos; reconstrucción de tablero y primera línea de Gato. | Implementado; con casos positivos y negativos. |
| §9: consultar resultados | GET /matches y GET /results, snapshots abiertos/cerrados y JSON estructurado. | Implementado; no requiere interfaz gráfica. |
| §10: desarrollo y entrega | Historial, PRs, versiones fijadas, Docker, CI/CD y despliegue integrado local. | Existe una versión funcional publicada. Falta incorporar estos ajustes nuevos por PR. |
| §12–13: separación y justificación de paradigmas | [Arquitectura](architecture.md) explica estado imperativo en Go y transformaciones funcionales en Scala, costes y extensibilidad. | Documentado; ambos integrantes deben poder defenderlo. |

## Alineación de capacidad incorporada

Antes Go podía guardar eventos y fallar posteriormente al consultar porque Scala
limitaba la colección a 10 000 eventos u 8 MiB. Ahora Go comprueba ambos límites
antes de aceptar el evento, usando el tamaño JSON real y una actualización
atómica por partida. Conserva espacio para el cierre: un evento y 1 MiB.
El exceso devuelve 413 y no modifica el historial ni consume el eventId.
No se borra ni recorta evidencia aceptada. [Política completa](integration-api.md#capacidad-y-reserva-para-cerrar).

La integración real comprobó snapshots abiertos y cerrados, una partida cerrada
de **10 000 eventos** y otra de **8 388 608 bytes exactos**, ambas analizadas por
Scala. También comprobó que los rechazos no alteran la partida y permiten cerrar.
El mismo procedimiento está incorporado al verificador Docker que ya ejecuta CI;
no se crea otro pipeline ni se publica otro paquete.

## Evidencia de esta revisión

Las suites Go de ambos módulos, vet, las 143 pruebas Scala, el verificador de
fixtures y las pruebas de infraestructura/distribución se ejecutaron durante
esta revisión. La compilación Scala se realizó con el lanzador CMD nuevo y JDK 21.
Los casos nuevos de capacidad pasan en Go. La ejecución Docker posterior a la
alineación terminó con:

- status=passed y containerIntegrationExecuted=true.
- Dos demostraciones: 100 eventos aceptados y 120 respuestas contrastadas.
- Tres comprobaciones negativas, seis duplicados rechazados por el motivo esperado
  y arranque con perfiles ausentes rechazado.
- Límites: eventBoundary=10000, byteBoundary=8388608,
  openAndClosedAnalyzed=true y rejectionWithoutMutation=true.
- resourcesReleased=true: se retiraron únicamente los recursos del proyecto de prueba.

La evidencia local detallada no se versiona. El detector de carreras para estos
ajustes debe aprobar en Linux mediante la CI del PR; las pruebas normales de
concurrencia locales no se presentan como ejecución del detector. Para la base
publicada, las evidencias remotas son [CI v1.0.1](https://github.com/JusMH/gamestats/actions/runs/39114936260)
y [publicación v1.0.1](https://github.com/JusMH/gamestats/actions/runs/39115113580).

## Límites que permanecen y no impiden por sí solos los mínimos

El enunciado no exige videojuegos completos, nube, interfaz gráfica, base de datos,
persistencia, caché, min/max, Prolog ni estadísticas históricas entre partidas.
Se conservan los límites documentados: memoria volátil, sin límite global de
partidas, análisis por consulta y timeout de cinco segundos. Gato reconoce la
evidencia acordada de líneas, sin certificar toda la legalidad de los turnos;
Snake no simula física. Son alcances que deben explicarse, no funciones completas
que puedan darse por implementadas.

La nueva alineación cubre **cantidad y bytes**. El parser JSON, roster, datos de
dominio y tamaño de resultados de Scala conservan sus validaciones independientes.
Un sobre aceptado por Go no garantiza datos semánticamente válidos. ARM64 y
prestaciones masivas no se declaran probadas.

## Qué procede antes de entregar

1. Revisar el diff y confirmar que solo incluye los ajustes autorizados.
2. Commit y push de avanceFran, PR hacia main y esperar todos los checks de CI.
3. Con CI aprobada y revisión del equipo, hacer merge. Una ejecución local correcta
   no reemplaza la validación remota del commit que se integrará.
4. Ensayar la demostración desde la versión elegida. Si se presentan los ajustes
   nuevos, ejecutar esas fuentes; el ZIP v1.0.1 no se actualiza al hacer merge.
   Crear otra Release es una decisión separada, no una exigencia nueva del enunciado.
5. Confirmar los nombres de integrantes, formato solicitado en TecDigital y defensa;
   adjuntar/enlazar los ocho entregables y realizar el envío administrativo.

El usuario confirmó como fecha real **domingo 18 de octubre de 2026**; el enunciado
escribe «Setiembre». Se conserva esa aclaración sin alterar el documento original.
El texto exige entrega antes de las **22:00 por TecDigital**. La recepción efectiva,
la composición del grupo, originalidad y aceptación docente requieren comprobación
humana: no se pueden deducir de que Docker o CI terminen correctamente.
