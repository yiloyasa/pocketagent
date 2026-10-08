import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatTimeline.kt'
s = io.open(p, encoding='utf-8').read()

old3 = '''                    }
                }
                // 思考过程:可折叠卡片(字号跟随"思考显示大小"设置);
                // 受全局"显示思考过程"开关控制,关闭时只看正文与结果
                "thinking" -> if (AgentConnection.showThinking) {'''
new3 = '''                    }
                }
                }
                // 思考过程:可折叠卡片(字号跟随"思考显示大小"设置);
                // 受全局"显示思考过程"开关控制,关闭时只看正文与结果
                "thinking" -> if (AgentConnection.showThinking) {'''
assert s.count(old3) == 1, f'e3: {s.count(old3)}'
s = s.replace(old3, new3)

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
assert s.count(old4) == 1, f'e4: {s.count(old4)}'
s = s.replace(old4, new4)

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
assert s.count(old5) == 1, f'e5: {s.count(old5)}'
s = s.replace(old5, new5)

old_i1 = 'import dev.pocketagent.app.ui.component.PaSkeletonList\n'
assert s.count(old_i1) == 1
s = s.replace(old_i1, old_i1 + 'import dev.pocketagent.app.ui.component.paEditorialShadow\n')
old_i2 = 'import dev.pocketagent.app.ui.theme.PaEditorial\n'
assert s.count(old_i2) == 1
s = s.replace(old_i2, old_i2 + 'import dev.pocketagent.app.ui.theme.PaGlassToken\n')

io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('remaining edits applied')
