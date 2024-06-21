import axios from 'axios';

export interface GitHubCallbackResp {
    token: string;
}

export function githubCallback(code: string) {
    return axios.get<GitHubCallbackResp, GitHubCallbackResp>('/auth/github/callback', {params: {code}});
}


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
