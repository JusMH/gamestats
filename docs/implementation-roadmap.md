# MVP académico y hoja de implementación

## Estado actual verificable

| Etapa | Estado | Evidencia |
| --- | --- | --- |
| Go 1–3: ingestión y coordinación | Implementado | Sobre v1, memoria por juego/partida, lifecycle, concurrencia, snapshots y puente HTTP bajo demanda. |
| Scala 4A: motor funcional | Implementado | Perfiles declarativos, count/sum/avg, duración, composición y HTTP. |
| Scala 4B: patrones y especialización | Implementado | Ventanas temporales, tablero Gato y 13 fixtures completos, motor puro/HTTP/integración. |
| Simulador 5 | Implementado | Seis partidas propias, tres juegos, productores HTTP, fases concurrentes y verificación independiente en vivo/final. |
| Docker y Compose 6 | Implementado y verificado | Tres imágenes, salud, red interna, tarea finita y verificación; estado ejecutado en docker.md. |
| CI 7: GitHub Actions | Implementado; ejecución remota pendiente | Go/race, Scala, contratos, integración Docker y artefactos; ver README del pipeline. |
| CD y deployment | Pendiente | Requiere decisiones de registro, infraestructura y credenciales. |

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
| Integración continua | Workflow implementado y validado localmente; primera ejecución GitHub pendiente. |
| CD y deployment funcional | Pendientes. |
| Documentación y defensa | Contratos, perfiles, fixtures, README e informes reproducibles locales. |

El MVP académico completo aún requiere validar CI en GitHub, CD, despliegue y su validación final.
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

Esto no acredita una ejecución remota de GitHub Actions: queda pendiente
subir la rama y abrir el PR por decisión humana. Tras la primera ejecución,
revisar logs, artefactos de 14 días, healthchecks y resourcesReleased=true.
La limitación histórica del host Windows para -race permanece en ese host;
la prueba sí se ejecutó en Linux durante esta etapa y es obligatoria en CI.

## Próxima tarea: publicación y deployment, previa decisión de infraestructura

1. Completar la primera ejecución CI remota y resolver cualquier diferencia
   del runner sin ocultar fallos ni desactivar verificaciones.
2. Decidir plataforma, registro, política de versiones y retención de imágenes.
3. Acordar credenciales de mínimo privilegio y separación CI/CD.
4. Diseñar deployment y verificación de salud/análisis en esa infraestructura.
5. Resolver la volatilidad de Go y las políticas de acceso según el alcance
   aprobado; no asumir persistencia o producción a partir de Compose local.

Se propone CI result como check obligatorio, pero no se modificaron reglas
de main. No hay publicación de imágenes ni deployment implementados.

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