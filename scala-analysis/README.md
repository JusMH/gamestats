# Análisis funcional — Scala

Estado: módulo real pendiente de implementar. La Etapa 1 define contratos,
perfiles y fixtures; no contiene un servidor Scala ejecutable.

Scala recibirá POST /analyze con snapshots abiertos o cerrados y devolverá
métricas y patrones mediante funciones puras, sin almacenar acumuladores
entre consultas. Go seguirá a cargo de ingestión, estado y concurrencia.

- [Contrato HTTP](../docs/integration-api.md).
- [Capacidades componibles y perfiles](../docs/analysis-templates.md).
- [Resultados provisionales/finales y orden temporal](../docs/metrics-and-patterns.md).
- [Datos de Gato, Snake y racing](../docs/games.md).
- [Fixtures de contrato](../docs/fixtures/analysis-v1/README.md).
- [Siguiente implementación y aceptación](../docs/implementation-roadmap.md).

Primera tarea: servicio HTTP real, perfiles estáticos y capacidades puras
count/sum/avg y duración, verificadas con racing antes de añadir ventanas
Snake y el analizador de tablero Gato. No habrá condicionales por gameId
en el ejecutor común ni evaluación de scripts desde configuración.
La selección concreta de build y bibliotecas queda para esa tarea.