import { SearchComponent } from './../components/search/search.component';
import { computed, inject, Injectable, signal } from '@angular/core';
import { ENVIRONMENT } from '../enums';
import { AppService } from './app.service';

@Injectable({ providedIn: 'root' })
export class EnvironmentService {
  private appService = inject(AppService);

  searchEnvDev = signal(ENVIRONMENT.NONE);
  searchEnvTest = signal(ENVIRONMENT.NONE);
  fromEnv = signal(ENVIRONMENT.NONE);
  toEnv = signal(ENVIRONMENT.NONE);

  readonly isEnvSelectionEnabled = computed(() => !!this.appService.intent());
}
