# Fixtures deterministas del diseño analítico v1

Estos archivos son ejemplos de contrato, no salidas de Scala ejecutado.
Cada *.request.json es un cuerpo completo de POST /analyze, con eventos
aceptados en su orden original. Cada *.expected.json es la respuesta HTTP 200
que deberá producir el motor futuro según el perfil indicado en manifest.json.
No se envía el array completo a POST /events: para probar ingestión se envía
cada elemento de events por separado y en orden.

## Inventario y derivación

| Caso | Resultado esperado y fundamento |
| --- | --- |
| [ttt-win](ttt-win.request.json) | Alex coloca X en (0,0), (0,1), (0,2): 3 movimientos contra 2, total 5, duración 6 s; una línea ganadora. |
| [ttt-draw](ttt-draw.request.json) | 9 celdas distintas, ninguna línea: empate; Alex 5, Bea 4, total 9, duración 10 s. |
| [ttt-open](ttt-open.request.json) | Alex 2, Bea 1, total 3; tablero incompleto sin línea. Resultado provisional in_progress; duración unavailable/awaiting_finish. |
| [snake-positive](snake-positive.request.json) | p1: 4 manzanas, 100 puntos, promedio 25, vida 15 s; p2: 1, 5, promedio 5, vida 18 s. Duración 20 s. Una terna [a1,a2,a3], límite inclusivo de 10 s; a4 no reutiliza esos eventos. |
| [snake-negative](snake-negative.request.json) | p1: 3 manzanas, 60 puntos, promedio 20, vida 15 s. La terna abarca 11 s: cero patrones. p2: conteos/suma 0; promedio sin muestras y vida sin muerte unavailable. Duración 16 s. |
| [snake-invalid-number](snake-invalid-number.request.json) | Go acepta el sobre. p1 tiene 2 APPLE_EATEN; suma y promedio invalid/invalid_number, sin ignorar a1. Duración 5 s y vida 4 s válidas. No hay terna. |
| [race-start](race-start.request.json) | p1/p2: vueltas y suma 0; promedio unavailable/no_samples. Duración unavailable/awaiting_finish. |
| [race-live-2](race-live-2.request.json) | p1: 2 vueltas, suma 22000 ms, media 11000 ms; p2: 0 vueltas. Duración aún unavailable. Dos plantillas coexisten. |
| [race-live-3](race-live-3.request.json) | p1: 3 vueltas, suma 30000 ms, media 10000 ms; p2 sin muestras. La consulta repetida no acumula las 2 vueltas de la respuesta anterior. |
| [race-final](race-final.request.json) | Mismos 3 giros y 30000 ms que en vivo, ahora final; duración de partida 32 s, diferente de sumar vueltas. p2 sin muestras. |
| [unknown-game](unknown-game.request.json) | Sin perfil: metrics y patterns vacíos, HTTP 200 propuesto; Go sigue admitiendo data arbitrario. No se inventan métricas. |
| [snake-tied](snake-tied.request.json) | Tres eventos distintos a la misma hora forman una única terna [a3,a1,a2], en orden de aceptación, no por eventId. p1: 3 manzanas, 6 puntos, media 2; vida 4 s y duración 5 s. |
| [snake-missing-points](snake-missing-points.request.json) | Go acepta data {}. Conteos permanecen válidos; suma y promedio de p1 invalid/missing_field. No se sustituye el valor ausente por cero. |

Los archivos expected incluyen todas las filas, sujetos, estados, unidades,
procedencias y bases de snapshot; no se limitan a un extracto ilustrativo.
Las métricas numéricas sin muestras distinguen 0 de unavailable/no_samples.
Los casos inválidos mantienen el sobre y el ciclo de vida válidos: prueban
la separación entre ingestión Go y semántica analítica.

## Análisis vivo y final de racing

race-start, race-live-2, race-live-3 y race-final corresponden a la misma
partida race-live y son prefijos crecientes de su colección definitiva.
Deben consultarse tras aceptar 1,3,4 y 5 eventos, respectivamente.
No deben reingerirse sus prefijos como eventos nuevos: eso produciría 409.

| Snapshot | Vueltas p1 | Suma p1 | Media p1 | Duración de partida |
| --- | --- | --- | --- | --- |
| Inicio | 0 provisional | 0 provisional | unavailable/no_samples | unavailable/awaiting_finish |
| Dos vueltas | 2 provisional | 22000 ms provisional | 11000 ms provisional | unavailable/awaiting_finish |
| Tres vueltas | 3 provisional | 30000 ms provisional | 10000 ms provisional | unavailable/awaiting_finish |
| Cerrado | 3 final | 30000 ms final | 10000 ms final | 32 s final |

p2 sigue en el roster con cero vueltas/suma y sin media.
Repetir cualquiera de esos requests debe devolver el mismo objeto esperado.
El perfil combina event-metrics/v1 y duration/v1 sin analizador de carreras.
Los tiempos de vuelta vienen del productor; duración de partida viene de
dos timestamps. Mejor vuelta no se incluye: min queda como extensión.

## Verificación local de ejemplos

Desde la raíz del repositorio, con Python 3.10 o posterior:

```sh
python docs/fixtures/analysis-v1/check.py
```

Solo utiliza la biblioteca estándar. No instala dependencias, no llama a
red, no escribe eventos ni calcula estadísticas en Go. Comprueba:
- JSON sin claves repetidas y referencias de archivos correctas;
- perfiles y nueve mutaciones inválidas;
- sobre, ciclo de vida, sujetos, IDs y orden de resultados;
- procedencia y referencia del snapshot;
- aritmética de todos los valores esperados de estos fixtures;
- tablero ganador/empate, ternas positivas/negativas y empate temporal;
- distinción de muestras inválidas, promedios vacíos y duraciones ausentes.

El script es un oráculo acotado a estos ejemplos: no es el motor Scala,
un simulador, un validador universal de perfiles ni una integración HTTP.
No admite ejecutar scripts de configuración. No verifica el comportamiento
de un servicio inexistente ni todos los casos límite de la futura API.

## Pruebas futuras obligatorias adicionales

La implementación Scala deberá convertir estos archivos en pruebas de
contrato y agregar: roster inválido, playerId ausente/desconocido,
mínimos numéricos, fin repetido, timestamp anterior al inicio, muerte tras
cierre, respuestas invalid/unavailable de patrones, dos ganadores de Gato,
casilla ocupada, eventos reordenados y zonas equivalentes, ventanas
solapadas largas, precisión decimal y redondeo HALF_EVEN.
También probará añadir una métrica a un perfil sin recompilar y que un error
de una capacidad no anule las demás válidas.

Los fixtures no prueban notificaciones push, persistencia o entrega exactamente
una vez; esas garantías no existen en el sistema actual.
