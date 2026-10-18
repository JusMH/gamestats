# Contrato HTTP Go → Scala y análisis bajo demanda

Contrato implementado por Go y el servicio Scala real. Los elementos
interiores se especifican en metrics-and-patterns.md. Esta revisión
documental no cambia rutas, campos exteriores ni códigos del puente Go.

## Petición

POST /analyze, Content-Type: application/json, Accept: application/json.

```json
{
  "gameId": "racing",
  "matchId": "race-live",
  "state": "open",
  "events": [
    {
      "schemaVersion": 1,
      "eventId": "s",
      "timestamp": "2026-10-17T18:00:00Z",
      "gameId": "racing",
      "matchId": "race-live",
      "type": "MATCH_STARTED",
      "data": {"players":["p1","p2"]}
    }
  ]
}
```

Los cuatro campos son obligatorios. state es open o closed. events contiene
el snapshot completo en orden de aceptación, al menos MATCH_STARTED.
Scala valida sobre v1, IDs únicos, coincidencia de gameId/matchId,
inicio único primero, ausencia de cierre si open y cierre único último si
closed. No debe aceptar una mezcla de partidas ni un arreglo vacío como
partida real. Las peticiones estructuralmente incoherentes reciben HTTP 400
en Scala; Go actual lo transforma en 502.

Go conserva campos conocidos del sobre y el contenido de data sin convertir
números a float64. No conserva campos desconocidos del sobre ni igualdad
textual de espacios/escapes. Opcionales vacíos se omiten.
La validación propia de los juegos solo corresponde a Scala.

## Respuesta exterior existente

HTTP 200 con un solo objeto JSON:

```json
{
  "gameId": "racing",
  "matchId": "race-live",
  "state": "open",
  "metrics": [],
  "patterns": []
}
```

El ejemplo ilustra el exterior; un perfil conocido producirá los elementos
especificados en [metrics-and-patterns.md](metrics-and-patterns.md).
metrics y patterns son arrays obligatorios, nunca null.
gameId, matchId y state deben coincidir exactamente con el snapshot enviado,
aunque la partida cambie mientras se calcula.

Go valida esos campos exteriores y transmite los elementos interiores como
JSON opaco. No verifica sus estados, IDs, cantidades ni procedencia: esa semántica
corresponde a Scala.
Scala construye los elementos interiores mediante sus modelos tipados.
Los campos exteriores adicionales se aceptan pero Go los descarta:
por eso status, source, basis y reason se ubican dentro de cada resultado,
sin depender de metadata exterior nueva.

Go rechaza JSON mal formado, contenido adicional, tipos exteriores incorrectos,
IDs/estado distintos y respuestas mayores de 8 MiB.
No exige Content-Type de respuesta; sí valida el cuerpo JSON.

## Análisis en vivo y final

GET /results/{gameId}/{matchId} obtiene un snapshot y llama a Scala en
cada consulta. Funciona con open y closed. No dispara cierre, no bloquea
mutexes durante la red, no guarda resultados y no mantiene caché.

El cálculo es una función del snapshot y del perfil activo, sin
acumular resultados de consultas anteriores. availability determina qué
salidas son útiles en vivo y cuáles esperan el cierre. Los resultados
individuales diferencian provisional, final, unavailable e invalid.
Un snapshot closed no implica que toda métrica tenga datos suficientes.

Flujo de integración de un productor:
1. POST MATCH_STARTED y esperar accepted.
2. POST acontecimientos y esperar sus aceptaciones.
3. GET /results tras un acontecimiento relevante o mediante polling.
4. Continuar enviando eventos mientras open.
5. POST MATCH_FINISHED tras todos los demás eventos; consultar de nuevo.

Go no envía automáticamente al cerrar. Sin GET no hay actualización enviada
al cliente. No existen notificaciones push, SSE ni WebSockets; son opcionales
posteriores. No se prescribe un intervalo de polling sin medir capacidad.

## Juegos y errores de dominio

Un gameId sin perfil devuelve desde Scala HTTP 200, los mismos
identificadores/state y arrays vacíos. No se inventan estadísticas ni se
rechaza la ingestión de un juego desconocido.
El protocolo actual no distingue públicamente "perfil ausente" de "perfil
sin resultados": esto es deliberado para conservar el exterior.
Una mejora de diagnóstico requeriría acordar una extensión del protocolo.

Un perfil conocido con una muestra numérica errónea devuelve HTTP 200
y las salidas afectadas invalid, junto con las independientes válidas.
Un roster declarado inválido impide identificar sujetos: Scala devuelve
400 para ese snapshot, no filas con IDs inventados.
Configuraciones inválidas impiden arrancar Scala. Fallos internos no se
convierten en métricas vacías: Scala devuelve 500.

## Códigos públicos actuales de GET /results

| Situación | Código Go | Cuerpo |
| --- | --- | --- |
| Análisis con exterior válido | 200 | gameId, matchId, state, metrics, patterns |
| Partida inexistente, incluso sin Scala configurado | 404 | {"error":"match not found"} |
| Falta SCALA_SERVICE_URL | 503 | {"error":"analysis service is not configured"} |
| Fallo de conexión, HTTP de Scala distinto de 200, redirección, JSON/exterior inválido o respuesta demasiado grande | 502 | {"error":"analysis service failed"} |
| Timeout o deadline | 504 | {"error":"analysis service timed out"} |
| Método distinto de GET, incluido HEAD | 405 | Error JSON y Allow: GET |

Los errores específicos HTTP 400/422/500 que pudiera emitir Scala no se
propagan como esos códigos al consumidor: todos son 502 en Go actual.
No se modifica esa política en esta etapa. No se exponen mensajes internos,
direcciones ni stack traces del servicio Scala.
Si el cliente se desconecta, su contexto cancela la llamada saliente y Go
no intenta escribir una respuesta. HEAD no transporta cuerpo por la red.
Rutas incompletas conservan el 404 estándar del router.
GET /health, POST /events y GET /matches mantienen sus contratos.

## Configuración y límites existentes

SCALA_SERVICE_URL es URL base absoluta HTTP(S), p. ej. http://localhost:9000.
Go agrega /analyze y admite un prefijo de ruta. No admite credenciales,
query ni fragmento; una URL inválida impide arrancar.
Sin variable, los otros endpoints siguen funcionando.

El cliente reutilizable admite concurrencia, timeout total de 5 segundos
incluyendo lectura, cancelación del contexto, sin redirecciones y sin
reintentos automáticos. El análisis debe completarse dentro de ese presupuesto.
Cada snapshot copia todos los eventos y bytes de data bajo RLock y luego
libera los bloqueos; costo proporcional al tamaño de la partida.
Go limita cada partida a 10 000 eventos totales y 8 MiB del snapshot JSON
serializado. No hay persistencia ni límite global de partidas.

### Capacidad y reserva para cerrar

- Inicio y cierre cuentan dentro de los 10 000 eventos. Una partida abierta
  admite como máximo 9 999; el último lugar se reserva a MATCH_FINISHED.
- Antes del cierre, Go reserva 1 MiB más el separador para su evento final.
  Así una partida llena puede cerrarse sin eliminar eventos anteriores.
- MATCH_FINISHED admite hasta 1 MiB **serializado**, además del límite previo
  de 1 MiB por POST. Escapes JSON pueden hacer distintos esos tamaños.
- El cómputo incluye sobre, identificadores, estado, todos los eventos y comas.
  El estado closed es mayor que open y se utiliza para reservar el presupuesto.
- Si un evento excede capacidad, POST /events devuelve **413** con
  `{"status":"rejected","error":"..."}`. No se agrega al historial ni al
  índice de IDs; puede corregirse y reenviarse con el mismo eventId.
- Duplicados y errores del ciclo de vida conservan su prioridad y sus códigos.
  No se recortan eventos, dividen partidas ni borran partidas automáticamente.

El inicio también comprueba que los identificadores permitan un cierre mínimo
dentro de 1 MiB: eventId único de hasta 64 caracteres ASCII sencillos, timestamp
UTC de 20 caracteres, sin campos opcionales y data:{}. Ese es un cierre posible,
no un nuevo formato obligatorio para todos los eventos. En Gato/Snake/carreras
el cierre admite data:{} según [juegos](games.md).

Las constantes Go están en internal/matches/limits.go; corresponden a los límites
ya existentes de Decoding.scala y HttpService.scala. El verificador Docker prueba
partidas abiertas/cerradas de 10 000 eventos y una cerrada de exactamente 8 MiB
contra Scala real. Los límites JSON y la validación analítica de Scala siguen
siendo independientes; esta alineación no convierte Go en analizador de juegos.

## Verificación del contrato

El [catálogo de fixtures](fixtures/analysis-v1/README.md) incluye requests y
respuestas esperadas abiertas/cerradas. El verificador local comprueba
ejemplos y derivaciones; no sustituye pruebas contra Scala real.
Las pruebas Go existentes usan mocks de HTTP, no un motor Scala.
La suite Scala compara los 13 pares mediante el motor puro y HTTP real.
La integración Go → Scala y el simulador también se ejecutan con servicios
reales en CI; ver [auditoría y evidencia publicada](academic-audit.md).
Los mocks de las pruebas unitarias Go no sustituyen esa integración.
