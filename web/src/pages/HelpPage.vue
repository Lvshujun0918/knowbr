<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { appState, ensureMeta } from '../state'
import ModeBadge from '../components/ModeBadge.vue'

const meta = computed(() => appState.meta)

onMounted(() => {
  void ensureMeta()
})
</script>

<template>
  <div class="page page--narrow">
    <header class="page-head">
      <h1 class="page-title">使用说明</h1>
      <p class="page-lead">如何加载预置案例、完成一轮完整学习，以及本系统的能力边界。</p>
    </header>

    <div class="card">
      <h2 class="card__title">快速上手：加载预置案例</h2>
      <ol class="list list--ordered">
        <li>进入<RouterLink to="/submit">提交实验材料</RouterLink>页面，点击「加载演示案例」。</li>
        <li>选择一个案例（推荐「类别极不平衡的三分类任务」核心演示），点击「填入此案例」，所有字段会自动填好。</li>
        <li>也可以在首页点击「体验演示案例」，直接带着核心案例进入提交页。</li>
        <li>演示案例的数据是人工构造的教学数据，页面中会以「模拟案例」标签明确标注。</li>
      </ol>
    </div>

    <div class="card">
      <h2 class="card__title">完成一轮「诊断—辅导—迁移—反馈」</h2>
      <ol class="list list--ordered">
        <li><strong>提交材料</strong>：填写实验信息，「学生自己的实验解释」为必填；混淆矩阵可按表格填写或粘贴文本，系统会实时校验并计算指标。</li>
        <li><strong>学习诊断</strong>：点击「开始诊断」，查看主要问题、引用的证据、诊断理由与掌握情况。</li>
        <li><strong>分层辅导</strong>：先「给我一级提示」（只给观察方向），需要时再升级到二级提示或局部示例；用自己的话重新解释并提交，系统会反馈是否有所修正。</li>
        <li><strong>迁移测试</strong>：进入后阅读新情境题，先独立作答并提交，再查看各维度评分与理由，之后才能查看参考答案。</li>
        <li><strong>学习反馈</strong>：点击「完成本次学习」，查看结构化反馈，可复制或导出为 Markdown / TXT。</li>
      </ol>
    </div>

    <div class="card">
      <h2 class="card__title">当前运行模式</h2>
      <p v-if="meta" class="help-mode">
        <ModeBadge :mode="meta.mode" />
      </p>
      <p v-if="meta" class="help-mode-notice">{{ meta.mode.notice }}</p>
      <p v-else class="help-mode-notice">正在获取运行模式…</p>
    </div>

    <div class="card">
      <h2 class="card__title">已实现功能</h2>
      <ul class="list">
        <li>实验材料提交（10 个字段）与材料完整性检查。</li>
        <li>混淆矩阵实时校验与程序计算指标（准确率、逐类召回率/精确率/F1、宏平均与加权平均 F1）。</li>
        <li>基于证据的学习问题诊断（七类问题分类、典型误区识别、矛盾检测）。</li>
        <li>三级分层提示与学生修正解释反馈；多轮无进展时给出教师介入建议。</li>
        <li>迁移测试：新情境题、先作答后评分、分维度评分理由、作答后才显示参考答案。</li>
        <li>结构化学习反馈与教师摘要，支持复制与 Markdown / TXT 导出。</li>
        <li>学习记录列表、删除与清空；教师视图含共性问题统计（仅基于本机已保存记录）。</li>
        <li>七个预置演示案例（涵盖分类指标、模型结构、训练稳定性与环境排障），全过程不依赖外部服务即可演示。</li>
      </ul>
    </div>

    <div class="card">
      <h2 class="card__title">暂未实现功能</h2>
      <ul class="list">
        <li>真实班级管理、账号体系与学生身份管理（本期为单机演示系统）。</li>
        <li>执行学生提交的代码（系统只阅读你粘贴的代码文本，不会运行它）。</li>
        <li>PDF、Word、图片等文件上传与解析（本期仅支持文本输入）。</li>
        <li>未配置大模型 API Key 时，没有大模型自然语言生成，全部内容由内置规则模板产生（功能不缺失）。</li>
        <li>跨课程模块的知识库（首期仅聚焦机器学习分类实验）。</li>
      </ul>
    </div>

    <div class="card">
      <h2 class="card__title">已知限制</h2>
      <ul class="list">
        <li>规则模式下，诊断基于关键词与内置规则，可能误判或漏判；结论应结合教师判断使用。</li>
        <li>迁移测试评分为关键词覆盖的规则化反馈，不代表正式考试成绩或学习成效测量。</li>
        <li>学习记录保存在本机服务端 SQLite 数据库中，清理数据库即丢失；教师统计仅来自这些记录，不能外推为班级整体情况。</li>
        <li>演示案例数据为人工构造，仅用于教学演示。</li>
      </ul>
    </div>

    <div class="card">
      <h2 class="card__title">诚实原则</h2>
      <ul class="list">
        <li v-for="(line, i) in meta?.honesty ?? []" :key="i">{{ line }}</li>
      </ul>
    </div>
  </div>
</template>
