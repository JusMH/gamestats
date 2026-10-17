# Fuente de eventos simulados

Estado: simulador pendiente; esta etapa solo define ejemplos deterministas.
No se desarrollarán videojuegos completos ni una consola.

El futuro productor emitirá acontecimientos de Gato y Snake y una prueba
de carreras, con partidas intercaladas y consultas de resultados en vivo
y tras el cierre. El productor detecta movimientos, manzanas o vueltas;
GameStats analiza los eventos que recibe.

Usará el [sobre v1](../docs/events.md), los [datos de cada juego](../docs/games.md)
y los [fixtures](../docs/fixtures/analysis-v1/README.md) como base.
Enviará cada evento por POST /events esperando aceptación, inicio primero
y cierre después de todos los anteriores. No reenviará prefijos de snapshots
como eventos nuevos: los eventId repetidos reciben 409.

Podrá consultar GET /results tras eventos relevantes o mediante polling.
Sin consulta no habrá actualización automática ni notificaciones push.
Los datos inválidos del dominio tendrán sobre válido para comprobar la
separación entre ingestión Go y análisis Scala.