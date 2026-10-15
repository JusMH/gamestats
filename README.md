# GameStats

Plataforma multilenguaje para el procesamiento y análisis de eventos de videojuegos. Proyecto académico del curso de Lenguajes de Programación.

## Propósito

Recibir eventos de múltiples partidas y juegos, organizarlos y producir estadísticas y detecciones de patrones sin depender de la implementación de un videojuego específico.

## Componentes previstos

| Carpeta | Responsabilidad |
| --- | --- |
| `go-ingestion/` | Recepción, validación, agrupación, estado temporal y concurrencia en Go. |
| `scala-analysis/` | Transformación funcional, estadísticas y patrones en Scala. |
| `simulator/` | Fuente simulada de eventos para pruebas y demostración. |
| `docs/` | Arquitectura, contrato de eventos y decisiones técnicas. |
| `.github/workflows/` | CI/CD (por implementar cuando existan los módulos). |

## Estado del proyecto

**Implementación 3 de Go disponible.** El servicio HTTP valida el contrato v1, almacena eventos en memoria por juego y partida, controla inicio/cierre y duplicados, y permite consultar el estado de ingestión. Incluye pruebas automatizadas de validación, ciclo de vida y concurrencia. Incluye el mecanismo HTTP de integración bajo demanda con Scala mediante snapshots de partidas abiertas o cerradas. El módulo real Scala, el simulador, Docker, CI/CD y el despliegue todavía están pendientes. Ver la [propuesta de contrato de integración](docs/integration-api.md), pendiente de aprobación del equipo.

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
