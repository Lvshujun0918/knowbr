import { createRouter, createWebHistory } from 'vue-router'

const HomePage = () => import('../pages/HomePage.vue')
const SubmitPage = () => import('../pages/SubmitPage.vue')
const SessionPage = () => import('../pages/SessionPage.vue')
const RecordsPage = () => import('../pages/RecordsPage.vue')
const TeacherPage = () => import('../pages/TeacherPage.vue')
const HelpPage = () => import('../pages/HelpPage.vue')

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomePage },
    { path: '/submit', name: 'submit', component: SubmitPage },
    { path: '/session/:id', name: 'session', component: SessionPage, props: true },
    { path: '/records', name: 'records', component: RecordsPage },
    { path: '/teacher', name: 'teacher', component: TeacherPage },
    { path: '/help', name: 'help', component: HelpPage },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
