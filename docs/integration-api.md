# Contrato HTTP Go → Scala y análisis bajo demanda

El transporte exterior descrito ya existe en Go. Los elementos interiores
de métricas/patrones quedan definidos por este diseño para implementar Scala;
no se declara que el servicio analítico real esté disponible.
No cambia ninguna ruta, nombre exterior, estado ni código del puente Go.

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
El futuro Scala validará sobre v1, IDs únicos, coincidencia de gameId/matchId,
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
JSON opaco. No verifica todavía sus estados, IDs, cantidades ni procedencia.
Scala deberá comprobar el contrato interior antes de responder.
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

El cálculo futuro será una función del snapshot y del perfil activo, sin
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

Un gameId sin perfil devuelve desde el futuro Scala HTTP 200, los mismos
identificadores/state y arrays vacíos. No se inventan estadísticas ni se
rechaza la ingestión de un juego desconocido.
El protocolo actual no distingue públicamente "perfil ausente" de "perfil
sin resultados": esto es deliberado para conservar el exterior.
Una mejora de diagnóstico requeriría acordar una extensión del protocolo.

Un perfil conocido con una muestra numérica errónea devuelve HTTP 200
y las salidas afectadas invalid, junto con las independientes válidas.
Un roster declarado inválido impide identificar sujetos: Scala devolverá
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
reintentos automáticos. El futuro Scala debe respetar ese presupuesto.
Cada snapshot copia todos los eventos y bytes de data bajo RLock y luego
libera los bloqueos; costo proporcional al tamaño de la partida.
No hay límite de eventos acumulados ni persistencia.

## Verificación de contrato pendiente

El [catálogo de fixtures](fixtures/analysis-v1/README.md) incluye requests y
respuestas esperadas abiertas/cerradas. El verificador local comprueba
ejemplos y derivaciones; no sustituye pruebas contra Scala real.
Las pruebas Go existentes usan mocks de HTTP, no un motor Scala.
La implementación próxima deberá ejecutar esos mismos pares contra
POST /analyze y GET /results, incluyendo consultas repetidas y simultáneas.
