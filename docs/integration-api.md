# Integración HTTP Go → Scala — propuesta pendiente de aprobación

**Estado: propuesta inicial para revisión conjunta del equipo.** Esta
implementación proporciona únicamente el mecanismo de integración; requiere
el servicio real que desarrolla el integrante responsable de Scala.
No declara estable el contrato ni define métricas o patrones.

## Petición a Scala

`POST /analyze`, `Content-Type: application/json`, `Accept: application/json`.

```json
{
  "gameId": "snake",
  "matchId": "match-001",
  "state": "open",
  "events": []
}
```

`state` es `open` o `closed`. `events` contiene **todos los eventos aceptados
hasta el instante del snapshot**, en orden de aceptación, incluidos
`MATCH_STARTED` y, si corresponde, `MATCH_FINISHED`. El arreglo vacío ilustra
la estructura: una partida existente contiene al menos su inicio.
Cada evento conserva los campos del [sobre v1](events.md) y el JSON original
de `data`, incluidos números grandes y objetos anidados. Go utiliza los
campos conocidos del sobre; las extensiones desconocidas del sobre ya se
ignoraban al validar en la implementación 2. Los campos opcionales vacíos
se omiten al serializar. No se promete igualdad textual de espacios o escapes
JSON en el transporte; sí se conserva el contenido de `data` sin convertir
números a float64 ni interpretar su semántica.

## Respuesta de Scala

HTTP **200**, cuerpo JSON único:

```json
{
  "gameId": "snake",
  "matchId": "match-001",
  "state": "open",
  "metrics": [],
  "patterns": []
}
```

Los identificadores y `state` deben coincidir exactamente con el snapshot
enviado, aunque lleguen eventos o se cierre la partida durante el análisis.
`metrics` y `patterns` son arreglos obligatorios (no null); Scala definirá sus
elementos. Go conserva su JSON sin calcular estadísticas. Se admiten campos
adicionales en la respuesta, pero solo se devuelven estos cinco campos.
El cliente rechaza JSON mal formado, contenido adicional, tipos incorrectos,
campos requeridos ausentes y respuestas mayores de **8 MiB**. No exige una
cabecera Content-Type en la respuesta; valida el cuerpo como JSON.

## Consulta pública en Go

`GET /results/{gameId}/{matchId}` toma un snapshot y llama a Scala en cada
consulta. Funciona con partidas abiertas y cerradas. No hay caché ni envío
automático al cerrar la partida. El resumen `GET /matches/...` se conserva.

| Situación | Código Go | Cuerpo JSON |
| --- | --- | --- |
| Análisis correcto | 200 | Respuesta analítica validada |
| Partida inexistente, aun sin configurar Scala | 404 | `{"error":"match not found"}` |
| Falta SCALA_SERVICE_URL | 503 | `{"error":"analysis service is not configured"}` |
| Error de conexión, HTTP distinto de 200 (incluidos 3xx y errores de Scala), JSON o estructura inválidos | 502 | `{"error":"analysis service failed"}` |
| Timeout del cliente o deadline de la solicitud | 504 | `{"error":"analysis service timed out"}` |
| Método diferente de GET, incluido HEAD | 405 | JSON de error y `Allow: GET` |

No se transmiten direcciones internas, cuerpos de error ni excepciones de
Scala al consumidor. Si el consumidor se desconecta, su contexto cancela la
solicitud saliente y Go no intenta escribir una respuesta. No se fabrican
estadísticas cuando falla Scala. Las rutas incompletas conservan el 404
estándar del router.

## Configuración y límites

`SCALA_SERVICE_URL` es la URL **base**, por ejemplo `http://localhost:9000`.
Go agrega `/analyze`; admite un prefijo de ruta. Debe ser una URL absoluta
HTTP(S), sin credenciales, query ni fragmento. Una URL configurada inválida
impide iniciar el servidor con un mensaje genérico. Sin variable, los demás
endpoints siguen disponibles.

El cliente reutilizable es seguro para consultas concurrentes y usa un timeout
total de **5 segundos**, incluyendo lectura de respuesta. La cancelación del
GET se propaga mediante `NewRequestWithContext`. No sigue redirecciones ni
hace reintentos automáticos. Los límites de servidor Go existentes se conservan.

Cada consulta copia todos los eventos bajo el RLock de la partida, incluidos
los bytes de data. Luego libera todos los bloqueos antes de serializar o usar
la red. La copia tiene costo proporcional al tamaño acumulado y puede retrasar
brevemente escrituras de la misma partida. Dos consultas abiertas pueden
representar instantes distintos; los resultados no se guardan. Los eventos
siguen solo en memoria y se pierden al reiniciar. No se agregan autenticación,
persistencia, paginación, mensajería, Docker, CI/CD ni analizadores.

## Coordinación pendiente con Scala

- Aprobar endpoint, nombres, estados y estructura propuesta; esta salida usa
  `state`, mientras el ejemplo histórico de events.md usaba `status`.
- Definir elementos y semántica de metrics/patterns, y comportamiento para
  partidas abiertas o juegos desconocidos (sin resultados ficticios).
- Acordar orden por timestamp, desempates y tratamiento de secuencias parciales.
- Confirmar límites de tamaño, tiempo de análisis y política de errores.
- Ejecutar pruebas de contrato contra el módulo Scala real antes de declarar
  la integración completa.
