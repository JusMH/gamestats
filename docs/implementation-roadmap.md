# MVP académico y hoja de implementación

## Estado actual verificable

| Etapa | Estado | Evidencia |
| --- | --- | --- |
| Go 1–3: ingestión y coordinación | Implementado | Sobre v1, memoria por juego/partida, lifecycle, concurrencia, snapshots y puente HTTP bajo demanda. |
| Scala 4A: motor funcional | Implementado | Perfiles declarativos, count/sum/avg, duración, composición y HTTP. |
| Scala 4B: patrones y especialización | Implementado | Ventanas temporales, tablero Gato y 13 fixtures completos, motor puro/HTTP/integración. |
| Simulador 5 | Implementado | Seis partidas propias, tres juegos, productores HTTP, fases concurrentes y verificación independiente en vivo/final. |
| Docker y Compose 6 | Implementado y verificado | Tres imágenes, salud, red interna, tarea finita y verificación; estado ejecutado en docker.md. |
| CI/CD y deployment | Pendiente | CI/CD es la siguiente tarea; deployment requiere una etapa posterior. |

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
| CI/CD y deployment funcional | Pendientes. |
| Documentación y defensa | Contratos, perfiles, fixtures, README e informes reproducibles locales. |

El MVP académico completo aún requiere CI/CD, despliegue y su validación final.
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

## Próxima tarea precisa: CI/CD

1. Elegir el proveedor de CI y fijar runtimes compatibles con las versiones
   actuales, sin modificar contratos para simplificar el pipeline.
2. Ejecutar pruebas/vet Go de ingestión y simulador, pruebas Scala,
   verificador de fixtures y pruebas de configuración Docker.
3. Construir las imágenes y ejecutar docker/verify.py con un proyecto exclusivo
   de cada ejecución, conservando evidencia ante éxito o fallo.
4. Comprobar errores esperados por su razón, no solo por salida distinta de cero;
   mantener verificaciones de métricas y patrones habilitadas.
5. Garantizar limpieza al cancelar y no interferir con otros trabajos de CI.
6. Definir política de cachés/artefactos. Publicación de imágenes requiere
   acordar registro, versiones y credenciales; no inventarlos.

Criterio de salida: cada cambio ejecuta los checks de los tres módulos y una
integración real en contenedores, con evidencia accesible y fallo explícito
ante regresiones. Esta implementación no agrega workflows ni publica imágenes.
Deployment remoto y su validación quedan para una tarea posterior.

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