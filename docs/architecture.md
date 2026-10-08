# Arquitectura inicial de GameStats

## Objetivo

Procesar eventos generados por videojuegos sin acoplar la plataforma a un único juego.

## Flujo conceptual

```text
Fuente externa / simulador
           |
           v
      Go: ingestión
  - validar eventos
  - organizar partidas
  - coordinar concurrencia
           |
           v
     Scala: análisis
  - transformar colecciones
  - agregar estadísticas
  - detectar patrones
           |
           v
 Resultados estructurados
           |
           v
 Consulta o presentación
```

## Restricciones del enunciado

- Go y Scala se ejecutan como componentes independientes.
- La comunicación puede ser HTTP, archivos estructurados o mensajería; el equipo debe justificar y documentar su decisión.
- Debe existir una fuente simulada de eventos, sin implementar los videojuegos.
- Se exige contenerización Docker, ejecución integrada, CI/CD y deployment funcional.

## Decisiones aún abiertas

1. Juegos y eventos concretos.
2. Esquema común y versiones de eventos.
3. Transporte Go → Scala y tratamiento de errores.
4. Cuándo una partida está lista para analizarse.
5. Estadísticas, detección de patrones y formato de resultados.
6. Estrategia de pruebas, Docker, CI/CD y deployment.

Consultar `docs/events.md` para registrar el contrato de eventos cuando sea acordado.
