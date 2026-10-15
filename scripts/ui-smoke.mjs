#!/usr/bin/env bun
/**
 * 知行桥 · 交互式界面冒烟测试
 *
 * 用真实浏览器（Bun 内置 Bun.WebView）把「提交材料 → 诊断 → 分层辅导（含智能体自主决定）→
 * 迁移测试 → 学习反馈 → 学习记录」点击一遍，并断言关键界面状态。
 * 它验证的是「按钮点了真的有反应」，而不是只看接口返回。
 *
 * 依赖：Bun ≥ 1.4（内置 WebView）；需要本机已安装 Chrome/Chromium。
 * 用法：
 *   cd web && pnpm build            # 先产出 web/dist
 *   cd ../server && go run .        # 另开终端启动服务
 *   bun scripts/ui-smoke.mjs http://127.0.0.1:8080
 * 若 Bun 找不到 Chrome，用 BUN_CHROME_PATH 指定可执行文件。
 */

const base = (process.argv[2] ?? "http://127.0.0.1:8080").replace(/\/$/, "");

const results = [];
function check(name, ok, extra = "") {
  results.push({ name, ok: !!ok, extra });
  console.log(`${ok ? "✅" : "❌"} ${name}${extra ? ` — ${extra}` : ""}`);
}

import { existsSync, readdirSync } from "node:fs";

/** 找一个可用的 Chrome/Chromium：先看环境变量，再按常见路径与 Playwright 缓存探测。 */
function findChrome() {
  const explicit = process.env.BUN_CHROME_PATH;
  if (explicit && existsSync(explicit)) return explicit;
  for (const p of [
    "/usr/bin/google-chrome-stable",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/snap/bin/chromium",
  ]) {
    if (existsSync(p)) return p;
  }
  const cache = `${process.env.HOME}/.cache/ms-playwright`;
  if (existsSync(cache)) {
    for (const dir of readdirSync(cache)) {
      for (const rel of [
        "chrome-linux64/chrome",
        "chrome-linux/chrome",
        "chrome-headless-shell-linux64/chrome-headless-shell",
      ]) {
        const p = `${cache}/${dir}/${rel}`;
        if (existsSync(p)) return p;
      }
    }
  }
  return null;
}

const chromePath = findChrome();
const consoleMessages = [];
const view = new Bun.WebView({
  width: 1440,
  height: 1200,
  // 以 root 运行 Chrome 需要 --no-sandbox（常见于容器/CI）。
  backend: {
    type: "chrome",
    ...(chromePath ? { path: chromePath } : {}),
    argv: ["--no-sandbox", "--disable-dev-shm-usage"],
  },
  // 把页面控制台接到这里：界面报错不应该只在录屏里被发现。
  console: (type, ...args) => {
    if (type === "error" || type === "warning" || type === "warn") {
      consoleMessages.push(
        `${type}: ${args.map((a) => (typeof a === "string" ? a : JSON.stringify(a)?.slice(0, 200))).join(" ")}`,
      );
    }
  },
});
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const js = (code) => view.evaluate(code);
const bodyText = () => js("document.body.innerText");
const pathname = () => js("location.pathname");

async function waitFor(condJs, timeoutMs = 12000) {
  const t0 = Date.now();
  while (Date.now() - t0 < timeoutMs) {
    try {
      if (await js(`!!(${condJs})`)) return true;
    } catch {
      /* 导航过程中求值可能短暂失败，重试即可 */
    }
    await sleep(120);
  }
  return false;
}

/** 按可见文字点击按钮/链接（Vue 的 @click 会响应合成点击）。 */
async function clickText(text, scope = "button, a") {
  return await js(`(() => {
    const t = ${JSON.stringify(text)};
    const el = [...document.querySelectorAll(${JSON.stringify(scope)})]
      .find((e) => (e.textContent || "").trim().includes(t) && !e.disabled && e.offsetParent !== null);
    if (!el) return false;
    el.click();
    return true;
  })()`);
}

/** 给受 v-model 绑定的输入框赋值（必须走原生 setter + input 事件）。 */
async function setField(selector, value) {
  return await js(`(() => {
    const el = document.querySelector(${JSON.stringify(selector)});
    if (!el) return false;
    const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, "value").set.call(el, ${JSON.stringify(value)});
    el.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
  })()`);
}

const contains = async (t) => (await bodyText()).includes(t);

try {
  // 1. 首页
  await view.navigate(`${base}/`);
  check("首页加载", await waitFor(`document.body.innerText.includes("知行桥")`));
  check("首页展示覆盖范围卡片", await contains("已覆盖的教学场景"));
  check(
    "覆盖范围数字来自服务端",
    /7 个任务类型/.test(await bodyText()),
    (await bodyText()).match(/\d+ 个任务类型[^\n]*/)?.[0] ?? "",
  );

  // 2. 体验演示案例 → 提交页
  check("点击「体验演示案例」", await clickText("体验演示案例"));
  check("跳转到提交材料页", await waitFor(`location.pathname === "/submit"`));
  await waitFor(`document.querySelector("input") && document.querySelector("input").value.length > 0`);
  check(
    "演示案例已填入表单",
    (await js(`document.querySelector("input")?.value ?? ""`)).includes("分类模型评价实验"),
  );

  // 3. 开始诊断 → 会话页
  check("点击「开始诊断」", await clickText("开始诊断"));
  check("跳转到学习诊断步骤", await waitFor(`location.pathname.startsWith("/session/")`, 20000));
  check("诊断结果可见", await waitFor(`document.body.innerText.includes("逐条主张诊断")`, 20000));
  check("诊断引用矩阵计算证据", await contains("混淆矩阵计算"));

  // 4. 分层辅导 + 智能体自主决定
  check("点击「进入分层辅导」", await clickText("进入分层辅导"));
  check("辅导面板可见", await waitFor(`document.body.innerText.includes("辅导对话")`));
  check("智能体决策卡片可见", await waitFor(`document.body.innerText.includes("智能体决策")`, 15000));
  const hintsBefore = await js(`document.querySelectorAll(".chat__item--hint").length`);
  check("点击「让智能体决定下一步」", await clickText("让智能体决定下一步"));
  check(
    "智能体自动给出提示（无需学生点「给我一级提示」）",
    await waitFor(`document.querySelectorAll(".chat__item--hint").length > ${hintsBefore}`, 15000),
  );
  check("界面说明这是系统决定的", await contains("智能体已自动给出"));

  // 5. 提交修正解释
  check("填写修正解释", await setField("#revision-input", "B 和 C 都被预测为 A，说明模型没有识别出这两个类别，召回率为 0。"));
  check("点击「提交我的解释」", await clickText("我再想一想 / 提交我的解释"));
  check("出现修正反馈", await waitFor(`document.body.innerText.includes("有所修正")`, 15000));

  // 6. 迁移测试：作答前没有参考答案
  check("点击「进入迁移测试」", await clickText("进入迁移测试"));
  check("迁移题与评分维度可见", await waitFor(`document.body.innerText.includes("评分维度（作答前可见）")`, 15000));
  check("作答前不显示参考答案", !(await contains("查看参考答案")));

  check(
    "填写迁移题答案",
    await setField(
      "#transfer-answer",
      "正样本只占很小比例，全判为阴性也能有很高的准确率；主指标应看召回率，并用精确率控制误报，结合混淆矩阵权衡两类错误的代价后调整阈值。",
    ),
  );
  check("点击「提交答案」", await clickText("提交答案"));
  check("出现评分结果", await waitFor(`document.body.innerText.includes("评分结果")`, 20000));
  check("作答后才出现「查看参考答案」", await waitFor(`document.body.innerText.includes("查看参考答案")`, 10000));

  // 7. 完成学习 → 学习反馈
  check("点击「完成本次学习」", await clickText("完成本次学习"));
  check("学习反馈页出现", await waitFor(`document.body.innerText.includes("建议教师关注")`, 20000));
  check("反馈含学习目标与关键证据", (await contains("本次学习目标")) && (await contains("关键证据")));

  // 8. 学习记录：本次流程新建的记录应出现在列表中，然后删除它（先接管 confirm 对话框）
  await view.navigate(`${base}/records`);
  check(
    "学习记录页加载",
    await waitFor(`document.body.innerText.includes("记录保存说明") || document.body.innerText.includes("还没有学习记录")`),
  );
  // 列表是异步拉取的，必须等行渲染出来再计数，否则会误判为空
  check("本次流程的记录出现在列表", await waitFor(`document.querySelectorAll("table tbody tr").length > 0`, 12000));
  const rowsBefore = await js(`document.querySelectorAll("table tbody tr").length`);
  check("记录列表有数据", rowsBefore > 0, `${rowsBefore} 条`);
  await js(`window.confirm = () => true`);
  check("点击「删除」", await clickText("删除"));
  check(
    "删除后列表减少一行且计数同步",
    await waitFor(`document.querySelectorAll("table tbody tr").length === ${rowsBefore - 1}`, 12000),
  );

  // 9. 页面控制台不应出现报错
  check("页面控制台无报错", consoleMessages.length === 0, consoleMessages.slice(0, 3).join(" | "));
} catch (err) {
  check("执行未抛异常", false, String(err).slice(0, 300));
} finally {
  try {
    await view.close();
  } catch {
    /* 忽略关闭失败 */
  }
}

const failed = results.filter((r) => !r.ok);
console.log(`\n${results.length - failed.length}/${results.length} 项通过`);
if (failed.length > 0) {
  console.log("未通过：" + failed.map((f) => f.name).join("、"));
  process.exit(1);
}
console.log("界面冒烟测试全部通过");
