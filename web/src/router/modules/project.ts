export default [
    {
        path: '/projects',
        component: () => import ('@/layouts/Console.vue'),

        children: [
            {
                path: '',
                component: () => import ('@/pages/Projects.vue'),
                name: 'Projects',
                meta: {
                    auth: true,
                }
            },
            {
                path: '/:uid',
                component: () => import ('@/pages/ProjectView.vue'),
                name: 'ProjectView',
                meta: {
                    auth: true,
                }
            }
        ]
    }
]
