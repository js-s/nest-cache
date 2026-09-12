import { createRouter, createWebHistory } from 'vue-router'
import { ensureSession } from './auth'
import Login from './views/Login.vue'
import Register from './views/Register.vue'
import Summary from './views/Summary.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/summary' },
    { path: '/login', component: Login },
    { path: '/register', component: Register },
    { path: '/summary', component: Summary, meta: { requiresAuth: true } },
    // Unknown paths funnel through / (→ /summary, itself auth-guarded),
    // so guests land on /login and signed-in users on /summary.
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) {
    return true
  }
  return (await ensureSession()) ? true : '/login'
})

export default router
