# Consulta agregada para GameConsole

Extensión compatible: no modifica eventos v1 ni las rutas existentes.

`GET /dashboard` en Go toma snapshots consistentes de cada partida y los envía
a `POST /analyze-summary` en Scala como `{ "matches": [snapshot, ...] }`.
Scala produce `source`, `scope`, `consistency`, `totals`, `games` y `matches`.
`totals` y cada juego incluyen `started`, `active`, `finished` y `events`.
El conteo de instancias corresponde a MATCH_STARTED aceptados y únicos.
Finalizadas incluye abandono. Cada partida registrada se lista en `matches`.
Los juegos configurados aparecen aunque no tengan partidas. Scala suma métricas
numéricas aditivas por jugador (count y sum); nunca promedia promedios.
Duración finalizada suma solo duraciones finales válidas. `invalidSamples`
indica filas omitidas; el consumidor debe advertir cuando un total es parcial.

Los datos son de la vida del proceso Go: no hay persistencia entre reinicios.
La colección no es una transacción global: cada partida tiene snapshot propio.
El cuerpo agregado está limitado a 8 MiB; Go responde 413 al excederlo.
Las consultas son bajo demanda. No hay acumulación entre consultas ni push.
Errores de análisis producen 502 y falta de configuración produce 503.

GameConsole debe presentar estos valores sin recalcular las estadísticas.
