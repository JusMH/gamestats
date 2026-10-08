# Ingestión y procesamiento — Go

Este módulo recibirá eventos de una fuente externa (simulador o datos de prueba), validará su estructura básica, organizará los eventos por juego y partida, mantendrá el estado temporal y coordinará procesamiento concurrente.

También preparará los datos para enviarlos a Scala. **Las estadísticas y reglas analíticas no corresponden a este módulo.**

> Pendiente: decidir el formato de entrada, la interfaz de comunicación y la estructura del código.
