import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/pages/Login.vue'),
      meta: { layout: 'blank', public: true },
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/pages/Dashboard.vue'),
    },
    {
      path: '/apps',
      name: 'apps',
      component: () => import('@/pages/AppsList.vue'),
    },
    {
      path: '/apps/:name',
      name: 'app-detail',
      component: () => import('@/pages/AppDetail.vue'),
      props: true,
    },
    {
      path: '/templates',
      name: 'templates',
      component: () => import('@/pages/Templates.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/pages/Settings.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.bootstrapped) await auth.refresh()
  if (!to.meta.public && !auth.user) {
    return { name: 'login', query: { next: to.fullPath } }
  }
})

export default router
