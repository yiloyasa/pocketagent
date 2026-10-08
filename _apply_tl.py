import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatTimeline.kt'
s = io.open(p, encoding='utf-8').read()

# --- Edit 1: user branch wrapper + grouping vals + offset Row ---
old1 = '                "user" -> Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {\n'
new1 = '''                "user" -> {
                    // 【D3-2 同 author 连发分组】相邻两条用户消息视觉上并成一组
                    // (ChatGPT/Claude 的气泡语言):后续消息上提 40% 间距(14dp→8.4dp),
                    // 相接两侧的圆角收小(7dp)读作"同一段话的延续"。纯视觉:offset
                    // 只动摆放位置,LazyColumn 的排列/锚定/回收行为零变化,黑客终端
                    // 块(22dp 大留白的终端语言)不参与分组。
                    val prevIsUser = index > 0 && st.items[index - 1].role == "user"
                    val nextIsUser = index < st.items.lastIndex && st.items[index + 1].role == "user"
                    val bubbleShape = if (PaHacker.isHacker) {
                        RoundedCornerShape(6.dp)
                    } else {
                        RoundedCornerShape(
                            topStart = if (prevIsUser) 7.dp else 20.dp,
                            topEnd = 7.dp,
                            bottomEnd = if (nextIsUser) 7.dp else 20.dp,
                            bottomStart = if (nextIsUser) 7.dp else 20.dp,
                        )
                    }
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .offset(y = if (prevIsUser && !PaHacker.isHacker) (-5.6).dp else 0.dp),
                        horizontalArrangement = Arrangement.End,
                    ) {
'''
assert s.count(old1) == 1
s = s.replace(old1, new1)

# --- Edit 2: bubble shape token + glass highlight border ---
old2 = '''                                .background(
                                    if (PaHacker.isHacker) PaEditorial.PrimaryContainer else PaEditorial.Graphite,
                                    if (PaHacker.isHacker) RoundedCornerShape(6.dp)
                                    else RoundedCornerShape(20.dp, 7.dp, 20.dp, 20.dp),
                                )
                                .padding(horizontal = 13.dp, vertical = 11.dp),
'''
new2 = '''                                .background(
                                    if (PaHacker.isHacker) PaEditorial.PrimaryContainer else PaEditorial.Graphite,
                                    bubbleShape,
                                )
                                // D3-2 石墨气泡 0.5dp 玻璃高光边:同一块玻璃家族的亮边
                                // 语言(PaGlassToken.Highlight,"光打在上缘"),实底石墨
                                // 从"贴纸"变"材质"。黑客终端块保持旧语言。
                                .then(
                                    if (PaHacker.isHacker) Modifier
                                    else Modifier.border(0.5.dp, PaGlassToken.Highlight, bubbleShape),
                                )
                                .padding(horizontal = 13.dp, vertical = 11.dp),
'''
assert s.count(old2) == 1
s = s.replace(old2, new2)

# --- Edit 3: close the wrapper right after the user branch tail ---
# HEAD tail (user branch): badge rows close 817-822, Row close 823, then blank +
# "thinking" branch comment. Anchor on the thinking comment to stay branch-local.
old3 = '''                    }
                }
                // 流式思考:边想边展示,和落定的思考卡同款样式
                if (st.running && AgentConnection.showThinking && st.thinkingText.isNotEmpty()) {'''
new3 = '''                    }
                }
                }
                // 流式思考:边想边展示,和落定的思考卡同款样式
                if (st.running && AgentConnection.showThinking && st.thinkingText.isNotEmpty()) {'''
assert s.count(old3) == 1
s = s.replace(old3, new3)

# --- Edit 4: streaming fade comment + param ---
old4 = '''                        // 流式输出中:正文 + 行内小号三点跟随(仍在生成,比光标更自然)
                        // 【P1-22】喂节流快照 displayedStream,不再逐 delta 帧直喂全文
                        Column(Modifier.fillMaxWidth()) {
                            InAppMarkdown(
                                text = displayedStream,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                onOpenUrl = onOpenBrowser,
                            )'''
new4 = '''                        // 流式输出中:正文 + 行内小号三点跟随(仍在生成,比光标更自然)
                        // 【P1-22】喂节流快照 displayedStream,不再逐 delta 帧直喂全文
                        // 【D4-1】streaming=true:新长出来的块播 ~140ms fade-in
                        // (MarkdownStreamingFade,方案与节流管线的关系见其注释)
                        Column(Modifier.fillMaxWidth()) {
                            InAppMarkdown(
                                text = displayedStream,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                onOpenUrl = onOpenBrowser,
                                streaming = true,
                            )'''
assert s.count(old4) == 1
s = s.replace(old4, new4)

# --- Edit 5: unread pill shadow (D3-1) ---
old5 = '''                    }
                    .background(ChatAccentColor, RoundedCornerShape(14.dp))
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_chevron_down), null,'''
new5 = '''                    }
                    // D3-1 浅盘悬浮胶囊海拔:Card 档阴影让它"浮"在时间线之上;
                    // 深色盘(含黑客)保持既有平语言,不加班驳阴影
                    .then(
                        if (PaEditorial.IsDark) Modifier
                        else Modifier.paEditorialShadow(PaEditorial.Shadow.Card, RoundedCornerShape(14.dp)),
                    )
                    .background(ChatAccentColor, RoundedCornerShape(14.dp))
                    .padding(horizontal = 12.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_chevron_down), null,'''
assert s.count(old5) == 1
s = s.replace(old5, new5)

# --- imports ---
old_i1 = 'import dev.pocketagent.app.ui.component.PaSkeletonList\n'
new_i1 = 'import dev.pocketagent.app.ui.component.PaSkeletonList\nimport dev.pocketagent.app.ui.component.paEditorialShadow\n'
assert s.count(old_i1) == 1
s = s.replace(old_i1, new_i1)
old_i2 = 'import dev.pocketagent.app.ui.theme.PaEditorial\n'
new_i2 = 'import dev.pocketagent.app.ui.theme.PaEditorial\nimport dev.pocketagent.app.ui.theme.PaGlassToken\n'
assert s.count(old_i2) == 1
s = s.replace(old_i2, new_i2)

io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('all 5 edits + imports applied')
