import {createRouter, createWebHistory, RouteRecordRaw} from "vue-router";
import NProgress from 'nprogress';
import 'nprogress/nprogress.css';
import {useAppStore} from "@/store";

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

router.beforeEach((_to, _from, next) => {
    NProgress.start()

    const appStore = useAppStore()
    const token = appStore.token
    if (token && (_to.name === 'SignIn' || _to.name === 'GitHubCallback')) {
        next({name: 'Dashboard'})
    } else if (!token && _to.meta.auth) {
        next({name: 'SignIn'})
    }

    next()
    NProgress.done()
})
export default router
