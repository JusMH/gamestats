# GameStats

Plataforma multilenguaje para el procesamiento y análisis de eventos de videojuegos. Proyecto académico del curso de Lenguajes de Programación.

## Propósito

Recibir eventos de múltiples partidas y juegos, organizarlos y producir estadísticas y detecciones de patrones sin depender de la implementación de un videojuego específico.

## Componentes

| Carpeta | Responsabilidad |
| --- | --- |
| `go-ingestion/` | Recepción, validación, agrupación, estado temporal y concurrencia en Go. |
| `scala-analysis/` | Transformación funcional, estadísticas y patrones en Scala. |
| `simulator/` | Fuente simulada de eventos para pruebas y demostración. |
| `docs/` | Arquitectura, contrato de eventos y decisiones técnicas. |
| `.github/workflows/` | CI/CD (pendiente). |

## Estado del proyecto

**Implementación 3 de Go disponible.** El servicio HTTP valida el contrato v1, almacena eventos en memoria por juego y partida, controla inicio/cierre y duplicados, y permite consultar el estado de ingestión. Incluye pruebas automatizadas de validación, ciclo de vida y concurrencia. Incluye el mecanismo HTTP de integración bajo demanda con Scala mediante snapshots de partidas abiertas o cerradas. Scala 4B ofrece HTTP real, count/sum/avg, duración, ventanas temporales y el analizador de líneas de Gato; el simulador multijuego ya produce seis partidas concurrentes con verificaciones propias en vivo y al cerrar. Docker, CI/CD y el despliegue están pendientes. Ver el [contrato de integración](docs/integration-api.md) y el [diseño de plantillas componibles](docs/analysis-templates.md). Ver la [ejecución Scala](scala-analysis/README.md) y las [arquitectura y verificaciones 4B](docs/scala-4b.md): los 13 fixtures multijuego se prueban con el motor y el puente Go reales.

Consultar [arquitectura](docs/architecture.md) y [contrato de eventos](docs/events.md).

## Requisitos académicos principales

- Go controla la ingestión, validación y procesamiento concurrente.
- Scala implementa análisis con colecciones y operaciones funcionales.
- Ambos módulos son independientes y se comunican mediante un mecanismo documentado.
- Deben existir simulación realista, estadísticas por jugador y partida, análisis combinado y al menos una regla basada en secuencias.
- Se requieren Docker, CI/CD, ejecución integrada y una versión desplegada funcional.

## Desarrollo

Para ejecutar el servicio Go (Go 1.22 o posterior):

```sh
cd go-ingestion
go run ./cmd/server
```

El servidor escucha en el puerto 8080. Consultar [el README del módulo Go](go-ingestion/README.md) para los endpoints, ejemplos de Postman, políticas de estado y pruebas (`go test ./...`, `go vet ./...` y `go test -race ./...` cuando el entorno lo permita).

## Diseño de la Etapa 1

El núcleo permanece genérico. Gato y Snake son juegos principales de demostración; racing verifica la composición configurable de métricas y duración sin lógica específica en el núcleo.

- [Arquitectura](docs/architecture.md), [sobre v1](docs/events.md) e [integración HTTP](docs/integration-api.md).
- [Perfiles y plantillas](docs/analysis-templates.md), [datos de juegos](docs/games.md) y [resultados en vivo/finales](docs/metrics-and-patterns.md).
- [Fixtures verificables](docs/fixtures/analysis-v1/README.md) y [MVP y siguiente tarea de Docker Compose](docs/implementation-roadmap.md).

La consulta GET /results recalcula un snapshot bajo demanda, abierto o cerrado; no hay push ni análisis automático al cerrar. El motor 4B ya se implementó y probó con los 13 fixtures completos de Gato, Snake, racing y juego desconocido, incluyendo integración Go → Scala real.
## Simulador multijuego disponible

El [simulador Go independiente](simulator/README.md) genera dos partidas de Gato,
dos de Snake y dos de carreras, sin ejecutar videojuegos ni leer los fixtures.
Utiliza workers por partida, barreras y HTTP público; verifica métricas y patrones
contra expectativas propias en cinco fases, repitiendo las consultas.

Desde simulator, con Go y Scala iniciados:

~~~powershell
go run ./cmd/simulator --run-id defensa-01 --report evidence/defensa-01.json
~~~

Integración real ejecutada: 50 eventos aceptados en seis partidas y 60 consultas
verificadas (48 abiertas, 12 cerradas), con seis solicitudes simultáneas.
También se comprobaron los casos negativos y el rechazo al reutilizar IDs.
El README del simulador incluye arranque completo, script automatizado Windows,
pruebas y limitaciones. Siguiente tarea: contenerización con Docker Compose.