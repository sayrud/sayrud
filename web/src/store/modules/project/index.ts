import {defineStore} from 'pinia';
import {ProjectState} from './types';
import {type Project} from '@/api/project.ts';

const useProjectStore = defineStore('sayrud/project', {
    persist: true,

    state: (): ProjectState => ({
        currentProject: null
    }),

    actions: {
        setProject(project: Project) {
            this.currentProject = project;
        },

        cleanProject() {
            this.currentProject = null;
        }
    }
})

export default useProjectStore;
