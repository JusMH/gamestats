# MVP académico y hoja de implementación

## Estado actual verificable

| Etapa | Estado | Evidencia |
| --- | --- | --- |
| Go 1–3: ingestión y coordinación | Implementado | Sobre v1, memoria por juego/partida, lifecycle, concurrencia, snapshots y puente HTTP bajo demanda. |
| Scala 4A: motor funcional | Implementado | Perfiles declarativos, count/sum/avg, duración, composición y HTTP. |
| Scala 4B: patrones y especialización | Implementado | Ventanas temporales, tablero Gato y 13 fixtures completos, motor puro/HTTP/integración. |
| Simulador 5 | Implementado | Seis partidas propias, tres juegos, productores HTTP, fases concurrentes y verificación independiente en vivo/final. |
| Docker y Compose 6 | Implementado y verificado | Tres imágenes, salud, red interna, tarea finita y verificación; estado ejecutado en docker.md. |
| CI 7: GitHub Actions | Implementado y aprobado en GitHub (PR #11) | Go/race, Scala, contratos, integración Docker y artefactos; ver README del pipeline. |
| Distribución académica 8 | Preparada; publicación pendiente | Workflow Release → GHCR → ZIP con digests y prueba anónima; aún no entregable. |
| Deployment remoto | Fuera de la modalidad académica local | Requiere una decisión posterior de infraestructura. |

La implementación 5 no modificó lógica Go/Scala, perfiles ni fixtures.
Documentación operativa: [Go](../go-ingestion/README.md),
[Scala](../scala-analysis/README.md), [verificación 4B](scala-4b.md)
y [simulador](../simulator/README.md); ejecución de contenedores en [Docker](docker.md).

## Requisitos académicos cubiertos y pendientes

| Requisito | Evidencia actual |
| --- | --- |
| Ingestión, flujo, memoria y concurrencia Go | Pruebas de eventos, partidas, snapshots y HTTP. |
| Servicio Scala independiente | POST /analyze y GET /health; puente Go real. |
| Programación funcional | Transformaciones puras, agrupaciones, folds inmutables y errores explícitos. |
| Métricas configurables | count/sum/avg por jugador/partida, sin condicionales por juego en el motor. |
| Composición | Racing combina métricas/duración; Snake agrega ventanas; Gato agrega especialización. |
| Análisis multievento | Inicio-fin, inicio-muerte, ternas de manzanas y tablero reconstruido. |
| Secuencias | Ventana greedy temporal y primera línea ganadora. |
| Análisis abierto/final | GET /results repetidos sin acumulación; provisional/final/unavailable/invalid. |
| Fuente externa simulada | Módulo Go separado, dos partidas por juego y dos jugadores por partida. |
| Simultaneidad | Workers por partida y barreras; seis solicitudes HTTP simultáneas demostradas. |
| Pruebas de contratos e integración | 13 fixtures Scala; escenarios propios del simulador con servicios reales. |
| Docker y ejecución integrada reproducible | Tres Dockerfiles multietapa y Compose; verificación y resultados en docker.md. |
| Integración continua | CI aprobada en GitHub y fusionada en PR #11. |
| Distribución sin herramientas de desarrollo | Preparada; imágenes públicas y verificación completa pendientes. |
| Deployment remoto | No implementado. |
| Documentación y defensa | Contratos, perfiles, fixtures, README e informes reproducibles locales. |

La entrega local académica requiere publicar las imágenes, verificar acceso
anónimo y validar el ZIP completo. Si la rúbrica exige alojamiento remoto,
esa interpretación debe acordarse aparte; esta distribución no lo implementa.
Docker dispone de una etapa específica de verificación; no se da por concluida
la entrega académica por disponer de ejecución local.

## Verificación ejecutada en implementación 5

- Simulador: compilación y go vet correctos; 31 funciones de prueba,
  90 casos terminales aprobados, sin contar dos veces los subtests padre.
- Servidor Go: go test ./... y go vet ./... correctos.
- Scala: compilación y 143 pruebas aprobadas, incluidos los 13 fixtures.
- Verificador documental: 30 JSON, 13 pares y 9 mutaciones inválidas.
- Integración con procesos independientes: seis partidas, 50 eventos,
  60 consultas verificadas, 48 abiertas y 12 cerradas; pico de seis
  solicitudes simultáneas.
- Tres comprobaciones negativas y seis rechazos 409 al repetir el run-id.
- go test -race del simulador pendiente por entorno: CGO deshabilitado
  y ausencia de gcc/clang en PATH. No se declara aprobado.

El simulador genera escenarios propios y expectativas explícitas: no usa
los fixtures como un sustituto de sus productores. Las pruebas HTTP
guionizadas comprueban la orquestación; la integración ejecutada realiza
realmente los cálculos en Scala.

## Implementación 6: Docker Compose

Se agregaron imágenes independientes multietapa, bases oficiales fijadas por
versiones/digests y runtime sin privilegios. Scala se empaqueta con dockerStage:
app.jar más cinco dependencias, sin sbt en runtime. No cambian perfiles,
fixtures ni lógica Go/Scala/simulador.

Compose utiliza DNS interno, Go en loopback del anfitrión y Scala sin puerto
publicado. El simulador es una tarea bajo demanda con el perfil tools.
Los healthchecks reales y las condiciones service_healthy ordenan el arranque;
las consultas analíticas del simulador verifican el puente completo.

Integración real ejecutada en Linux/amd64: dos demos de seis partidas,
50 eventos y 60 consultas cada una; tres negativos, seis duplicados rechazados,
perfiles ausentes rechazados y limpieza completa del proyecto de pruebas.

La [guía Docker](docker.md) contiene resultados ejecutados y procedimiento
portable: construcción, salud, demo, negativos, duplicados, otro ID y limpieza
de un proyecto exclusivo. El script nunca declara aprobado un fallo del motor
ni borra recursos de otros proyectos. La evidencia es local e ignorada por Git.

## Implementación 7: integración continua

[GameStats CI](../.github/workflows/README.md) ejecuta trabajos independientes
Go (matriz de dos módulos), Scala, contratos/infraestructura e integración
Docker real; CI result resume y exige éxito de todos. Triggers: PR a main,
push a main y workflow_dispatch. Ubuntu 24.04, Go 1.26.5, Temurin 21, sbt
1.11.7 y Scala 3.3.7. Acciones oficiales fijadas por SHA, token de lectura,
sin secretos, publicación de imágenes ni cambios de protección de ramas.

Verificaciones locales de esta etapa: ambos módulos Go pasan build/test/vet y
-race en Linux con CGO=1 y GCC, sin carreras detectadas. Scala: compile/test/
dockerStage y 143 pruebas; artefacto con 13 fixtures; 16 pruebas de infraestructura;
verificador 30 JSON/13 pares/9 mutaciones; actionlint y guards de evidencia.
El verificador Docker real pasó dos demos 6/50/60, negativos, duplicados,
perfiles ausentes y limpieza en el proyecto exclusivo ci-local-20261017-7.

CI del PR #11 ya fue aprobada y fusionada. El detector de carreras se mantiene
obligatorio en CI; no se desactivan verificaciones al preparar publicación.

## Etapa 8: publicación y distribución académica

Se preparó un workflow independiente, activado manualmente desde main con SHA revisado y confirmación, con control
de main/CI exacta, permisos mínimos separados, rechazo de tags GHCR existentes,
labels OCI y registro de digests reales. El Compose de desarrollo no cambia.
El Compose de evaluación usa únicamente imágenes; el ZIP contiene configuración,
manuales y manifiesto, sin fuentes ni herramientas de desarrollo.

La [guía técnica](academic-distribution.md) documenta publicación inicial,
visibilidad pública manual, instalación del profesor y recuperación. Antes de
adjuntar el ZIP se exige descarga anónima e integración completa desde el paquete
extraído en un directorio temporal. La prueba conserva imágenes compartidas y
limpia solo sus contenedores/red. No existe publicación desde PR.

Verificado localmente: 24 pruebas específicas de distribución, actionlint,
Compose válido, 16 pruebas existentes y verificador documental. Se generó un
ZIP PRUEBA con digests ficticios identificados; no es material entregable.
Publicación GHCR, acceso público real, subida de adjuntos e instalación completa
del ZIP definitivo siguen pendientes de ejecutar después de revisión.

## Siguiente paso: primera Release académica y validación del evaluador

1. Revisar y fusionar la Etapa 8; esperar CI exitosa exacta del commit en main.
2. Iniciar manualmente el workflow desde main con v1.0.0, SHA revisado y
   confirmación PUBLICAR v1.0.0, sin crear tag/Release previamente.
3. Confirmar los tres paquetes GHCR públicos; cambiar visibilidad solo con
   autorización administrativa y reejecutar jobs fallidos si corresponde.
4. Exigir verificación anónima, demo/negativos/IDs y limpieza; solo entonces
   crear borrador con todos los adjuntos verificados y publicar la Release.
   El flujo admite Releases inmutables; no distribuir artefactos fallidos.
5. Validar la experiencia Docker Desktop del profesor. ARM64, funcionamiento
   offline y deployment remoto no forman parte de lo verificado.

No se crearon tags, Releases, commits, push ni paquetes durante esta etapa.

## Extensiones fuera del alcance actual

min/max para mejor vuelta, nuevas unidades, capacidades adicionales y nuevos
perfiles pueden estudiarse después; no están incluidos por ser sencillos.
Nuevos mapeos de eventos a count/sum/avg ya funcionan mediante perfiles.

No se incorporan ajedrez/damas completos, motor universal de reglas,
interfaces gráficas, registro dinámico por API, persistencia distribuida,
Kafka, torneos/equipos, procesamiento incremental avanzado ni WebSockets/SSE.

## Riesgos y decisiones pendientes

- Go retiene todos los eventos en memoria y no comparte estado entre procesos.
  Definir límites/retención antes de escalar; no asumir persistencia.
- No hay reintentos idempotentes, reapertura ni exactamente una vez.
  El simulador usa run-id distinto para nuevas demostraciones.
- Snapshots completos y polling tienen costo creciente. Medir antes de
  introducir caché, procesamiento incremental o mayor frecuencia.
- La versión activa del perfil puede cambiar resultados incluso cerrados;
  source.profileVersion debe permanecer visible y las versiones ser inmutables.
- El puente traduce fallos HTTP de Scala a 502; diagnósticos públicos más
  específicos requerirían una tarea separada, no modificaciones en el simulador.
- Gato verifica la evidencia acordada, sin certificar todas las reglas.
  Si la rúbrica exige más legalidad, deberá acordarse explícitamente.
- La detección de carreras Go debe ejecutarse en un entorno con CGO y
  compilador C compatibles; las pruebas concurrentes actuales no la sustituyen.
- La plataforma de despliegue, credenciales y políticas de acceso deben
  acordarse al preparar deployment, sin inventar infraestructura ahora.