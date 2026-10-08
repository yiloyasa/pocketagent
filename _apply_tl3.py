import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatTimeline.kt'
s = io.open(p, encoding='utf-8').read()

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
assert s.count(old1) == 1, f'e1 {s.count(old1)}'
s = s.replace(old1, new1)

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
assert s.count(old2) == 1, f'e2 {s.count(old2)}'
s = s.replace(old2, new2)

io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('edits 1+2 applied')
