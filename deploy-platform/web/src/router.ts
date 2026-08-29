import { createRouter, createWebHistory } from 'vue-router'
import OverviewView from './views/OverviewView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: OverviewView, meta: { title: '发布总览' } },
    { path: '/apps', component: () => import('./views/AppsView.vue'), meta: { title: '应用' } },
    { path: '/apps/:id', component: () => import('./views/AppDetailView.vue'), meta: { title: '应用详情' } },
    { path: '/deployments', component: () => import('./views/DeploymentsView.vue'), meta: { title: '发布任务' } },
    { path: '/deployments/:id', component: () => import('./views/DeploymentDetailView.vue'), meta: { title: '发布详情' } },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
