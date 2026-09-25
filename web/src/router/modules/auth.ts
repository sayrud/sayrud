export default [
    {
        path: '/profile',
        component: () => import ('@/layouts/Console.vue'),
        children: [
            {
                path: '',
                component: () => import ('@/pages/auth/Profile.vue'),
                name: 'Profile',
                meta: {
                    auth: true,
                }
            }
        ]
    }
]
