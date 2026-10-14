<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { appState, ensureMeta } from './state'
import ModeBadge from './components/ModeBadge.vue'

const route = useRoute()

onMounted(() => {
  void ensureMeta()
})
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <div class="app-header__inner">
        <RouterLink to="/" class="app-brand" aria-label="知行桥首页">
          <span class="app-brand__mark" aria-hidden="true">桥</span>
          <span class="app-brand__name">知行桥</span>
          <span class="app-brand__sub">人工智能基础课程教学衔接智能体</span>
        </RouterLink>
        <nav class="app-nav" aria-label="主导航">
          <RouterLink to="/" class="app-nav__link" :class="{ 'is-active': route.path === '/' }">首页</RouterLink>
          <RouterLink to="/submit" class="app-nav__link" :class="{ 'is-active': route.path === '/submit' }">提交材料</RouterLink>
          <RouterLink to="/records" class="app-nav__link" :class="{ 'is-active': route.path === '/records' }">学习记录</RouterLink>
          <RouterLink to="/teacher" class="app-nav__link" :class="{ 'is-active': route.path === '/teacher' }">教师视图</RouterLink>
          <RouterLink to="/help" class="app-nav__link" :class="{ 'is-active': route.path === '/help' }">使用说明</RouterLink>
        </nav>
        <div class="app-header__mode">
          <ModeBadge v-if="appState.meta" :mode="appState.meta.mode" />
        </div>
      </div>
    </header>
    <main class="app-main" id="main">
      <RouterView />
    </main>
    <footer class="app-footer">
      <div class="app-footer__inner">
        <span>知行桥 · 《人工智能基础（A）》教学演示系统</span>
        <span v-if="appState.meta" class="app-footer__storage">{{ appState.meta.storage.location }}</span>
      </div>
    </footer>
  </div>
</template>
