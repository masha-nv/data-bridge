import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { EnvironmentService } from './environment.service';

export type DevopsJobStatus = 'active' | 'completed';

export interface DevopsJobRecord {
  statusCode: number;
  jobId: number;
  inFilePath: string;
  inFileUri: string;
  threadKey: string;
  batchName: string;
  batchTitle: string;
  threadPoolId: number;
  threadId: number;
  startDateTime: string;
  statusDateTime: string;
  createdDateTime: string;
  endDateTime: string;
}

export interface DevopsJobsResponse {
  environment: string;
  status: DevopsJobStatus;
  rows: DevopsJobRecord[];
}

export interface DevopsActionResponse {
  environment: string;
  returnCode: number;
  returnMessage: string;
}

@Injectable({ providedIn: 'root' })
export class DevopsService {
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  getJobs(
    environment: string,
    status: DevopsJobStatus,
  ): Observable<DevopsJobsResponse> {
    return this.httpClient.post<DevopsJobsResponse>(
      this.environmentService.getApiUrl('/api/marx/devops/jobs'),
      {
        environment,
        status,
      },
    );
  }

  restartJobs(
    environment: string,
    jobIds: string,
  ): Observable<DevopsActionResponse> {
    return this.httpClient.post<DevopsActionResponse>(
      this.environmentService.getApiUrl('/api/marx/devops/restart-jobs'),
      {
        environment,
        jobIds,
      },
    );
  }

  markJobsComplete(
    environment: string,
    currentStatus: string,
    jobIds: string,
  ): Observable<DevopsActionResponse> {
    return this.httpClient.post<DevopsActionResponse>(
      this.environmentService.getApiUrl('/api/marx/devops/mark-jobs-complete'),
      {
        environment,
        currentStatus,
        jobIds,
      },
    );
  }
}