package gamestats

import com.fasterxml.jackson.core.{JsonFactory, JsonParser, StreamReadConstraints}
import com.fasterxml.jackson.databind.{DeserializationFeature, JsonNode, ObjectMapper}
import scala.jdk.CollectionConverters.*
import scala.util.control.NonFatal

/** Immutable JSON boundary; no mutable Jackson node enters the engine. */
enum Json:
  case Obj(fields: Map[String, Json])
  case Arr(values: Vector[Json])
  case Str(value: String)
  case IntNum(value: BigInt)
  case DecNum(value: BigDecimal)
  case Bool(value: Boolean)
  case Null

case class ValidationError(code: String, path: String):
  def under(prefix: String): ValidationError = copy(path = prefix + "." + path)

type Checked[A] = Either[ValidationError, A]

object Json:
  private val factory = new JsonFactory()
  factory.setStreamReadConstraints(StreamReadConstraints.builder()
    .maxNestingDepth(64).maxNumberLength(128).maxStringLength(65536).build())
  factory.enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
  private val mapper = new ObjectMapper(factory)
    .enable(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS)
    .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS)

  def parse(bytes: Array[Byte]): Checked[Json] =
    try
      val node = mapper.readTree(bytes)
      if node == null || node.isMissingNode then Left(ValidationError("invalid_json", "body"))
      else Right(fromNode(node))
    catch case NonFatal(_) => Left(ValidationError("invalid_json", "body"))

  def parse(text: String): Checked[Json] =
    parse(text.getBytes(java.nio.charset.StandardCharsets.UTF_8))

  private def fromNode(n: JsonNode): Json =
    if n.isObject then Obj(n.properties().asScala.iterator.map(e => e.getKey -> fromNode(e.getValue)).toMap)
    else if n.isArray then Arr(n.elements().asScala.map(fromNode).toVector)
    else if n.isTextual then Str(n.textValue())
    else if n.isIntegralNumber then IntNum(BigInt(n.bigIntegerValue()))
    else if n.isFloatingPointNumber then
      val decimal = n.decimalValue()
      require(math.abs(decimal.scale().toLong) <= 1000 && decimal.precision() <= 128)
      DecNum(BigDecimal(decimal.toString, java.math.MathContext.UNLIMITED))
    else if n.isBoolean then Bool(n.booleanValue())
    else Null

  private def toNode(j: Json): JsonNode = j match
    case Obj(fields) =>
      val obj = mapper.createObjectNode()
      fields.toVector.sortBy(_._1).foreach { (key, value) => obj.set[JsonNode](key, toNode(value)) }
      obj
    case Arr(values) =>
      val array = mapper.createArrayNode()
      values.foreach(v => array.add(toNode(v)))
      array
    case Str(value) => mapper.getNodeFactory.textNode(value)
    case IntNum(value) => mapper.getNodeFactory.numberNode(value.bigInteger)
    case DecNum(value) => mapper.getNodeFactory.numberNode(value.bigDecimal)
    case Bool(value) => mapper.getNodeFactory.booleanNode(value)
    case Null => mapper.getNodeFactory.nullNode()

  def bytes(value: Json): Array[Byte] = mapper.writeValueAsBytes(toNode(value))

  /** Stops output before a large configured roster can allocate an unbounded response. */
  def boundedBytes(value: Json, limit: Int): Option[Array[Byte]] =
    val buffer = new java.io.ByteArrayOutputStream()
    val bounded = new java.io.OutputStream:
      override def write(value: Int): Unit =
        if buffer.size() >= limit then throw new java.io.IOException("response limit")
        buffer.write(value)
      override def write(bytes: Array[Byte], offset: Int, length: Int): Unit =
        if length > limit - buffer.size() then throw new java.io.IOException("response limit")
        buffer.write(bytes, offset, length)
    try
      mapper.writeValue(bounded, toNode(value))
      Some(buffer.toByteArray)
    catch case _: java.io.IOException => None

  def text(value: Json): String = new String(bytes(value), java.nio.charset.StandardCharsets.UTF_8)
  def num(value: BigDecimal): Json =
    if value.isWhole then IntNum(value.toBigInt) else DecNum(value)

  def obj(value: Json): Checked[Map[String, Json]] = value match
    case Obj(fields) => Right(fields)
    case _ => Left(ValidationError("expected_object", "object"))
  def arr(value: Json): Checked[Vector[Json]] = value match
    case Arr(values) => Right(values)
    case _ => Left(ValidationError("expected_array", "array"))
  def str(value: Json): Checked[String] = value match
    case Str(text) if text.trim.nonEmpty => Right(text)
    case _ => Left(ValidationError("expected_nonempty_string", "string"))
  def number(value: Json): Checked[BigDecimal] = value match
    case IntNum(n) => Right(BigDecimal(n, java.math.MathContext.UNLIMITED))
    case DecNum(n) => Right(n)
    case _ => Left(ValidationError("invalid_number", "number"))
  def integer(value: Json): Checked[Int] = value match
    case IntNum(n) if n.isValidInt => Right(n.toInt)
    case _ => Left(ValidationError("expected_integer", "integer"))
  def field(fields: Map[String, Json], name: String): Checked[Json] =
    fields.get(name).toRight(ValidationError("missing_field", name))
  def read[A](fields: Map[String, Json], name: String)(decode: Json => Checked[A]): Checked[A] =
    field(fields, name).flatMap(decode).left.map(_.under(name))
  def optional[A](fields: Map[String, Json], name: String)(decode: Json => Checked[A]): Checked[Option[A]] =
    fields.get(name) match
      case None => Right(None)
      case Some(value) => decode(value).map(Some(_)).left.map(_.under(name))
  def traverse[A, B](values: Vector[A])(f: A => Checked[B]): Checked[Vector[B]] =
    values.foldLeft[Checked[Vector[B]]](Right(Vector.empty))((acc, value) =>
      for xs <- acc; x <- f(value) yield xs :+ x)
  def ensure(condition: Boolean, code: String, path: String): Checked[Unit] =
    Either.cond(condition, (), ValidationError(code, path))
  def strict(value: Json, required: Set[String], optional: Set[String] = Set.empty): Checked[Map[String, Json]] =
    obj(value).flatMap(fields =>
      ensure(required.subsetOf(fields.keySet) && fields.keySet.subsetOf(required ++ optional),
        "invalid_fields", "object").map(_ => fields))

  /** Object-key JSON Pointer only, as defined by the profiles. */
  def pointer(value: Json, path: String): Option[Json] =
    path.drop(1).split("/", -1).foldLeft(Option(value)) { (current, token) =>
      val key = token.replace("~1", "/").replace("~0", "~")
      current.flatMap {
        case Obj(fields) => fields.get(key)
        case _ => None
      }
    }
