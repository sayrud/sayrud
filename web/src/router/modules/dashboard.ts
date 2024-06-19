export default [
    {
        path: '/dashboard',
        component: () => import ('@/layouts/Console.vue'),
        children: [
            {
                path: '',
                component: () => import ('@/pages/Dashboard.vue'),
                name: 'Dashboard'
            }
        ]
    }
]
