package gamestats

class SummarySuite extends munit.FunSuite:
  import TestData.*
  def request(ids: String*): Json = Json.Obj(Map("matches" -> Json.Arr(ids.toVector.map(fixture(_, "request")))))
  test("empty summary includes all configured games with zero instances") {
    val result = right(Summary.analyze(request(), engine))
    assertEquals(field(field(result,"totals"),"started"), Json.IntNum(0))
    assertEquals(array(field(result,"games")).size, 3)
  }
  test("open and closed snapshots count once and repeated queries never accumulate") {
    val input = request("race-final", "ttt-open", "snake-positive")
    val result = right(Summary.analyze(input, engine))
    assertEquals(field(field(result,"totals"),"started"), Json.IntNum(3))
    assertEquals(field(field(result,"totals"),"active"), Json.IntNum(1))
    assertEquals(field(field(result,"totals"),"finished"), Json.IntNum(2))
    assertEquals(right(Summary.analyze(input, engine)), result)
    val racing = array(field(result,"games")).find(g => field(g,"gameId") == Json.Str("racing")).get
    val laps = array(field(racing,"metrics")).find(m => field(m,"id") == Json.Str("laps.player")).get
    assertEquals(field(laps,"value"), Json.num(BigDecimal(3)))
  }
  test("duplicate match snapshots are rejected rather than counted twice") {
    assert(Summary.analyze(request("race-final","race-final"), engine).isLeft)
  }
