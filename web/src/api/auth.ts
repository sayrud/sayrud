import axios from 'axios';

export interface UserProfileResp {
    uid: string;
    userName: string;
    githubID: string;
    email: string;
    emailMd5: string;
}

export function userProfile() {
    return axios.get<UserProfileResp, UserProfileResp>('/auth/profile');
}
