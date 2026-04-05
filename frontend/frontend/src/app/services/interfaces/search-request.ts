import { ENVIRONMENT } from './../../enums';

export interface SearchRequest {
  envs: ENVIRONMENT[];
  table: string;
  value: any;
}

export interface MoveResponse {
  moved: number;
}
