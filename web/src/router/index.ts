import {createRouter, createWebHistory, RouteRecordRaw} from "vue-router";
import NProgress from 'nprogress';
import 'nprogress/nprogress.css';

NProgress.configure({showSpinner: false});

const moduleRoutes = import.meta.glob('./modules/**/*.ts', {eager: true})
const moduleRouteList: Array<RouteRecordRaw> = mapModuleRouterList(moduleRoutes);

export const allRoutes: Array<RouteRecordRaw> = [
    ...moduleRouteList,
]

export function mapModuleRouterList(modules: Record<string, unknown>): Array<RouteRecordRaw> {
    const routerList: Array<RouteRecordRaw> = [];
    Object.keys(modules).forEach((key) => {
        // @ts-ignore
        const mod = modules[key].default || {};
        const modList = Array.isArray(mod) ? [...mod] : [mod];
        routerList.push(...modList);
    });
    return routerList;
}

const router = createRouter({
    history: createWebHistory(),
    routes: allRoutes,
    scrollBehavior() {
        return {el: '#app', top: 0, behavior: 'smooth'}
    }
})

router.beforeEach((to, _from, next) => {
    NProgress.start()
    next()
    NProgress.done()
})
export default router
