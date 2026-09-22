package dev.pocketagent.app.ui

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 选择卡保真度(F-B)纯函数回归(AUDIT §4):
 * - questions 解析保留每个 option 的 description/preview 与问题级 multiSelect/isSecret/isOther;
 * - 提交打包:多选=answers 数组(按渲染顺序),单选/自由输入=旧形状(单元素数组);
 * - 旧 daemon 形状(选项只有 label、问题无新键)逐字段等价,不回归;
 * - 认不出的形状忽略不炸,选项全丢的题降级为自由输入;没答案就不放键,不伪造已答。
 */
class AskQuestionCardFidelityTest {

    private fun ask(vararg questions: JSONObject): JSONObject =
        JSONObject().put("questions", JSONArray().apply { questions.forEach { put(it) } })

    // ---- 解析:选项级 description / preview ----

    @Test
    fun parse_keeps_option_description_and_preview_when_present() {
        val args = ask(
            JSONObject()
                .put("id", "q1")
                .put("question", "选一个方案")
                .put(
                    "options",
                    JSONArray()
                        .put(
                            JSONObject().put("label", "方案A")
                                .put("description", "改动最小")
                                .put("preview", "fn a() {\n  return 1\n}"),
                        )
                        .put(JSONObject().put("label", "方案B")),
                ),
        )
        val qs = parseAskQuestions(args)
        assertEquals(1, qs.size)
        val opts = qs[0].options
        assertEquals(2, opts.size)
        assertEquals("方案A", opts[0].label)
        assertEquals("改动最小", opts[0].description)
        assertEquals("fn a() {\n  return 1\n}", opts[0].preview) // 原样保留,不改写换行
        // 没带副文案/预览的选项:空串=渲染层不画,不伪造占位
        assertEquals("方案B", opts[1].label)
        assertEquals("", opts[1].description)
        assertEquals("", opts[1].preview)
    }

    // ---- 解析:问题级 multiSelect / isSecret / isOther ----

    @Test
    fun parse_keeps_question_level_flags() {
        val qs = parseAskQuestions(
            ask(
                JSONObject().put("id", "m").put("question", "多选")
                    .put("multiSelect", true).put("isOther", true)
                    .put("options", JSONArray().put(JSONObject().put("label", "a"))),
                JSONObject().put("id", "s").put("question", "密码")
                    .put("isSecret", true),
            ),
        )
        assertEquals(2, qs.size)
        assertTrue(qs[0].multiSelect)
        assertTrue(qs[0].isOther)
        assertFalse(qs[0].isSecret)
        assertTrue(qs[1].isSecret)
        assertFalse(qs[1].multiSelect)
        assertFalse(qs[1].isOther)
    }

    // ---- 解析:未知字段/未知形状忽略不炸(README §7 能透传就透传) ----

    @Test
    fun parse_unknown_fields_and_unknown_option_shapes_are_ignored_not_fatal() {
        val q = JSONObject()
            .put("id", "q")
            .put("question", "?")
            .put("futureKey", JSONObject().put("x", 1)) // daemon 将来新增的未知键
            .put(
                "options",
                JSONArray()
                    .put(JSONObject().put("label", "ok"))
                    .put(42) // 非对象非字符串的选项形状:丢弃,不硬造
                    .put("裸字符串选项当label"), // 宽容:当 label 收下
            )
        val qs = parseAskQuestions(ask(q))
        assertEquals(listOf("ok", "裸字符串选项当label"), qs[0].options.map { it.label })
    }

    @Test
    fun parse_non_object_question_entry_is_skipped() {
        val arr = JSONArray().put("not-an-object").put(JSONObject().put("id", "q2").put("question", "真问题"))
        val qs = parseAskQuestions(JSONObject().put("questions", arr))
        assertEquals(listOf("q2"), qs.map { it.id })
    }

    @Test
    fun parse_no_questions_returns_empty() {
        assertEquals(0, parseAskQuestions(JSONObject().put("message", "纯文字反问")).size)
        assertEquals(0, parseAskQuestions(null).size)
    }

    // ---- 解析:旧形状(仅 label)不回归 ----

    @Test
    fun parse_old_label_only_shape_matches_legacy_behavior() {
        val q = JSONObject()
            .put("id", "t")
            .put("question", "旧形状")
            .put(
                "options",
                JSONArray().put(JSONObject().put("label", "x")).put(JSONObject().put("label", "y")),
            )
        val got = parseAskQuestions(ask(q))[0]
        assertEquals("t", got.id)
        assertEquals("旧形状", got.text)
        assertEquals(listOf("x", "y"), got.options.map { it.label })
        assertEquals(listOf("", ""), got.options.map { it.description })
        assertEquals(listOf("", ""), got.options.map { it.preview })
        assertFalse(got.multiSelect)
        assertFalse(got.isSecret)
        assertFalse(got.isOther)
    }

    // ---- 打包:多选提交 answers 数组 ----

    @Test
    fun pack_multiselect_submits_answers_array_in_render_order() {
        val qs = listOf(
            AskQuestion(
                id = "m",
                text = "多选",
                options = listOf(AskOption("b"), AskOption("a"), AskOption("c")),
                multiSelect = true,
            ),
        )
        val obj = packAskAnswers(qs, picks = mapOf("m" to setOf("a", "b")), texts = emptyMap())
        // 数组按选项渲染顺序(b,a,c)输出勾中项,不随 Set 无序迭代漂移
        assertEquals(listOf("b", "a"), obj.getJSONObject("m").getJSONArray("answers").map { it as String })
    }

    // ---- 打包:单选/自由输入维持旧形状 ----

    @Test
    fun pack_single_select_and_free_input_keep_legacy_shape() {
        val qs = listOf(AskQuestion(id = "s", text = "单选", options = listOf(AskOption("x"))))
        val obj = packAskAnswers(qs, picks = mapOf("s" to setOf("x")), texts = mapOf("s" to "x"))
        val arr = obj.getJSONObject("s").getJSONArray("answers")
        assertEquals(1, arr.length())
        assertEquals("x", arr.getString(0))
        // 纯输入题(无选项):trim 后单元素数组
        val free = packAskAnswers(
            listOf(AskQuestion(id = "f", text = "?", options = emptyList())),
            emptyMap(),
            mapOf("f" to " 手打 "),
        )
        assertEquals("手打", free.getJSONObject("f").getJSONArray("answers").getString(0))
    }

    @Test
    fun pack_missing_id_falls_back_to_stable_qidx_key() {
        val qs = listOf(AskQuestion(id = "", text = "无id", options = listOf(AskOption("z"))))
        val obj = packAskAnswers(qs, picks = mapOf("q0" to setOf("z")), texts = emptyMap())
        assertTrue(obj.has("q0"))
    }

    // ---- 打包:没答案就不放键,不伪造已答 ----

    @Test
    fun pack_unanswered_question_is_omitted_not_faked() {
        val qs = listOf(
            AskQuestion(id = "a", text = "1", options = listOf(AskOption("x"))),
            AskQuestion(id = "b", text = "2", options = emptyList()),
        )
        val obj = packAskAnswers(qs, picks = mapOf("a" to setOf("x")), texts = mapOf("b" to "  "))
        assertEquals(setOf("a"), obj.keys().asSequence().toSet())
    }

    @Test
    fun pack_multiselect_typed_text_appends_and_dedupes() {
        val qs = listOf(
            AskQuestion(
                id = "m",
                text = "多选",
                options = listOf(AskOption("a"), AskOption("b")),
                multiSelect = true,
            ),
        )
        val obj = packAskAnswers(qs, picks = mapOf("m" to setOf("a")), texts = mapOf("m" to " 其他 "))
        assertEquals(listOf("a", "其他"), obj.getJSONObject("m").getJSONArray("answers").map { it as String })
        // 输入与已选重复:不重复入数组
        val dup = packAskAnswers(qs, picks = mapOf("m" to setOf("a")), texts = mapOf("m" to "a"))
        assertEquals(1, dup.getJSONObject("m").getJSONArray("answers").length())
    }

    @Test
    fun pack_single_select_typed_text_replaces_pick_via_ui_contract() {
        // UI 契约:单选题打字会清空勾选(picks 已清、texts 有值)→ 只剩输入,不会双答案
        val qs = listOf(AskQuestion(id = "s", text = "单选", options = listOf(AskOption("a"))))
        val obj = packAskAnswers(qs, picks = emptyMap(), texts = mapOf("s" to "手打"))
        assertEquals(listOf("手打"), obj.getJSONObject("s").getJSONArray("answers").map { it as String })
    }

    // ---- 提交门槛(提交按钮可用性) ----

    @Test
    fun has_answers_gates_submit_button() {
        val qs = listOf(AskQuestion(id = "a", text = "1", options = listOf(AskOption("x"))))
        assertFalse(askHasAnswers(qs, emptyMap(), emptyMap()))
        assertTrue(askHasAnswers(qs, mapOf("a" to setOf("x")), emptyMap()))
        assertTrue(askHasAnswers(qs, emptyMap(), mapOf("a" to "答案")))
        // 纯空白输入不算答案
        assertFalse(askHasAnswers(qs, emptyMap(), mapOf("a" to " ")))
        // 纯文字卡(questions 缺失)恒可提交(旧行为)
        assertTrue(askHasAnswers(emptyList(), emptyMap(), emptyMap()))
    }

    // ---- 未知形状降级:自由输入兜底 ----

    @Test
    fun unknown_option_shape_degrades_to_free_input_only() {
        // 选项全丢(未知形状)→ options 为空 → 渲染层只剩自由输入;提交走输入形状
        val qs = parseAskQuestions(
            ask(JSONObject().put("id", "u").put("question", "?").put("options", JSONArray().put(7))),
        )
        assertEquals(1, qs.size)
        assertTrue(qs[0].options.isEmpty())
        val obj = packAskAnswers(qs, picks = emptyMap(), texts = mapOf("u" to "兜底输入"))
        assertEquals("兜底输入", obj.getJSONObject("u").getJSONArray("answers").getString(0))
    }

    // ---- F-A 契约终版逐字段对照 ----

    @Test
    fun contract_zcode_card_omitempty_shape() {
        // zcode/pi 卡:questions:[{id,question,options:[{label,description?,preview?}],multiSelect?:true}]
        // description/preview 空时键省略;multiSelect 仅 true 时出现,缺省=单选
        val zcode = ask(
            JSONObject()
                .put("id", "q0")
                .put("question", "选方案")
                .put(
                    "options",
                    JSONArray()
                        .put(JSONObject().put("label", "A").put("description", "描述在").put("preview", "code"))
                        .put(JSONObject().put("label", "B")), // description/preview omitempty 省略
                )
                .put("multiSelect", true), // true 才透出
            JSONObject().put("id", "q1").put("question", "单选题")
                .put("options", JSONArray().put(JSONObject().put("label", "x"))), // 无 multiSelect 键
        )
        val qs = parseAskQuestions(zcode)
        assertTrue(qs[0].multiSelect)
        assertFalse(qs[1].multiSelect) // 键缺省=false,不炸
        assertFalse(qs[0].isSecret)
        assertFalse(qs[0].isOther)
        assertEquals("", qs[1].options[0].description)
        // 提交仍走推荐形状 {qid:{answers:[...]}}(daemon 四形状兼容之一,不发明新形状)
        val obj = packAskAnswers(qs, picks = mapOf("q0" to setOf("A")), texts = mapOf("q1" to "y"))
        assertEquals(listOf("A"), obj.getJSONObject("q0").getJSONArray("answers").map { it as String })
        assertEquals(listOf("y"), obj.getJSONObject("q1").getJSONArray("answers").map { it as String })
    }

    @Test
    fun contract_codex_card_header_and_always_present_flags() {
        // codex 卡:questions:[{id,header,question,isSecret:bool,isOther:bool,options:[{label,description}]}]
        // isSecret/isOther 恒出现(false 也透出)
        val codex = ask(
            JSONObject()
                .put("id", "q0")
                .put("header", "修改方案")
                .put("question", "怎么改?")
                .put("isSecret", false)
                .put("isOther", false)
                .put("options", JSONArray().put(JSONObject().put("label", "方案A").put("description", "小改"))),
            JSONObject()
                .put("id", "q1")
                .put("header", "凭据")
                .put("question", "输入令牌")
                .put("isSecret", true)
                .put("isOther", false),
        )
        val qs = parseAskQuestions(codex)
        assertEquals("修改方案", qs[0].header)
        assertEquals("凭据", qs[1].header)
        assertFalse(qs[0].isSecret) // 恒出现且为 false → 单选常规渲染
        assertFalse(qs[0].isOther)
        assertTrue(qs[1].isSecret) // → 渲染密码态输入
        assertEquals("小改", qs[0].options[0].description)
    }

    @Test
    fun contract_header_absent_renders_nothing() {
        // header 键缺省(zcode 卡/旧 daemon):空串=渲染层不画小标题,占位不伪造
        val qs = parseAskQuestions(
            ask(JSONObject().put("id", "q0").put("question", "?").put("options", JSONArray().put(JSONObject().put("label", "a")))),
        )
        assertEquals("", qs[0].header)
    }

    // ---- 反问卡与 agent 无关(任务一 · 通用性契约) ----
    // 反问卡只认 action.name,不看 agent 身份。本测试把它锁死:这两类动作名恒走
    // "取消/提交"反问卡;其余(批准/权限类)一律不走反问卡。能力由 daemon 运行时上报
    // APPROVAL_REQUEST 来宣告 —— UI 不写死任何 agent 名单,因此推广到 codex/pi/dsh
    // 不需要改这里,各自 daemon 上报同类请求即自动复用同一张卡(无假入口)。

    @Test
    fun askAction_gates_only_request_user_input_and_mcp_elicitation() {
        assertTrue(askIsAskAction("request_user_input"))
        assertTrue(askIsAskAction("mcp_elicitation"))
    }

    @Test
    fun askAction_is_false_for_every_non_ask_action_name() {
        // 批准/权限/文件类动作都不该被误渲染成反问卡
        listOf("shell", "file_change", "apply_patch", "permissions", "write_file", "", "unknown")
            .forEach { assertFalse("动作 '$it' 不应走反问卡", askIsAskAction(it)) }
    }
}
