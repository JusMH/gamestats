# Arquitectura híbrida — GameStats (propuesta v1)

## Objetivo

Recibir eventos de múltiples juegos con un núcleo reutilizable; calcular métricas sencillas mediante configuración y patrones complejos mediante analizadores especializados, **respetando las responsabilidades obligatorias de Go y Scala**.

## Flujo de datos

```text
Simulador / productor externo
         |
         | HTTP POST /events (JSON)
         v
Go: ingestión y orquestación
  - valida sobre común
  - detecta duplicados por partida
  - agrupa por (gameId, matchId)
  - mantiene estado temporal
  - atiende partidas concurrentes con goroutines/channels
         |
         | HTTP POST /analyze (colección JSON al cerrar la partida)
         v
Scala: análisis funcional
  - recibe y transforma colecciones
  - aplica configuraciones declarativas (count, sum, avg)
  - utiliza analizadores especializados para patrones
  - devuelve métricas y hallazgos estructurados
         |
         | JSON
         v
Go: guarda temporalmente el resultado
         |
         | GET /results/{gameId}/{matchId}
         v
Cliente / Postman
```

Las rutas HTTP son **propuestas de diseño**, no rutas requeridas literalmente por el enunciado. Go y Scala corren como servicios independientes; no acoplamos tipos específicos de juegos al módulo Go.

## Contrato de eventos

Ver [events.md](events.md). El sobre incluye `schemaVersion`, `eventId`, `timestamp`, `gameId`, `matchId`, `type` y `data`; `playerId` y `gameVersion` son opcionales. `data` puede contener objetos JSON diferentes para cada juego.

## Responsabilidades

| Componente | Hace | No hace |
| --- | --- | --- |
| Simulador | Genera eventos variados, juegos y partidas en paralelo | No implementa videojuegos completos |
| Go | Recibe, valida sobre, mantiene estado, coordina concurrencia y comunicación | No calcula estadísticas del dominio |
| Scala | Valida semántica analítica, transforma colecciones, calcula estadísticas y detecta patrones | No coordina la ingestión externa |
| Configuraciones | Definen métricas simples por juego sin recompilar el motor | No ejecutan expresiones de código arbitrario |
| Analizadores Scala | Implementan reglas específicas de secuencias/correlaciones | No obligan a cambiar el núcleo genérico de Go |

## Enfoque híbrido de análisis

1. **Métricas declarativas**: un JSON por juego, cargado por Scala, establece `count`, `sum` o `avg` sobre eventos seleccionados. La implementación del motor será funcional, con operaciones como `filter`, `map`, `groupBy` y `fold`.
2. **Analizadores especializados**: Scala registra lógica programada para condiciones más expresivas, por ejemplo daño seguido de eliminación al mismo objetivo dentro de una ventana temporal.
3. **Juegos desconocidos**: Go admite eventos cuyo sobre sea válido, incluso sin configuración de Scala. No inferimos métricas específicas sin semántica definida. Como respaldo, Scala podrá devolver conteos genéricos por tipo de evento.

## Concurrencia y límites de la v1

- Go debe procesar partidas independientes en paralelo. Diseñaremos un administrador de partidas con estado protegido (por mutex o propietarios exclusivos mediante channels), evitando condiciones de carrera.
- Inicialmente Go mantiene eventos y resultados en memoria; no prometemos persistencia ante reinicio.
- El simulador enviará los eventos de cada partida antes de su `MATCH_FINISHED`. Entre partidas distintas, las llegadas pueden intercalarse.
- Go finaliza el lote por partida y lo envía a Scala; Scala puede ordenar timestamps para las reglas de secuencia.
- Al principio, reintentos HTTP, tolerancia a fallos y datos tardíos se resolverán con una política sencilla documentada, sin inventar garantías de entrega exactamente una vez.

## Estrategia de implementación incremental

1. Congelar la primera propuesta del contrato mediante revisión y casos de prueba.
2. Crear servicio Go mínimo: `POST /events`, validación del sobre y un endpoint de salud.
3. Incorporar estado por partida, duplicados y concurrencia en Go.
4. Crear servicio Scala: `POST /analyze` con una métrica `count` y una `sum` configurables.
5. Incorporar regla de secuencia en Scala y datos de prueba de combate.
6. Extender la simulación a carreras para comprobar que no necesitamos alterar Go.
7. Añadir consulta de resultados, manejo de errores, pruebas integradas, Docker Compose, CI/CD y deployment.

## Aspectos del enunciado que cubriremos

- Go: flujo, validación, estado y concurrencia.
- Scala: análisis funcional, métricas por jugador y partida y patrones de varios eventos.
- Múltiples juegos, partidas y escenarios simulados.
- Servicios independientes comunicados por HTTP/JSON.
- Docker, integración reproducible, CI/CD y deployment documentado.

**Nota:** Este documento representa una propuesta técnica del equipo; no sustituye al enunciado original y podrá evolucionar tras las primeras pruebas.
