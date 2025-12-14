import asyncio
import aiohttp
import time
import statistics
from collections import defaultdict
from datetime import datetime
import json

class LoadTester:
    def __init__(self, base_url, num_users=1000, duration=60):
        self.base_url = base_url
        self.num_users = num_users
        self.duration = duration
        self.results = {
            'total_requests': 0,
            'successful_requests': 0,
            'failed_requests': 0,
            'response_times': [],
            'server_distribution': defaultdict(int),
            'status_codes': defaultdict(int),
            'errors': []
        }
        
    async def make_request(self, session, endpoint):
        """Make a single HTTP request"""
        start_time = time.time()
        try:
            async with session.get(f"{self.base_url}{endpoint}", timeout=10) as response:
                response_time = (time.time() - start_time) * 1000  # Convert to ms
                data = await response.json()
                
                self.results['total_requests'] += 1
                self.results['response_times'].append(response_time)
                self.results['status_codes'][response.status] += 1
                
                if response.status == 200:
                    self.results['successful_requests'] += 1
                    if 'server_id' in data:
                        self.results['server_distribution'][data['server_id']] += 1
                else:
                    self.results['failed_requests'] += 1
                    
                return True
                
        except asyncio.TimeoutError:
            self.results['failed_requests'] += 1
            self.results['errors'].append('Timeout')
            return False
        except Exception as e:
            self.results['failed_requests'] += 1
            self.results['errors'].append(str(e))
            return False
    
    async def user_session(self, user_id, session, endpoints):
        """Simulate a single user making multiple requests"""
        end_time = time.time() + self.duration
        request_count = 0
        
        while time.time() < end_time:
            # Randomly select an endpoint
            endpoint = endpoints[request_count % len(endpoints)]
            await self.make_request(session, endpoint)
            request_count += 1
            
            # Small delay between requests (simulating user think time)
            await asyncio.sleep(0.1)
    
    async def run_test(self):
        """Run the load test"""
        print(f"\n{'='*60}")
        print(f"Starting Load Test")
        print(f"{'='*60}")
        print(f"Target URL: {self.base_url}")
        print(f"Concurrent Users: {self.num_users}")
        print(f"Duration: {self.duration} seconds")
        print(f"Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'='*60}\n")
        
        # API endpoints to test
        endpoints = [
            '/api/products',
            '/api/orders',
            '/api/users',
            '/'
        ]
        
        start_time = time.time()
        
        # Create aiohttp session with connection pooling
        connector = aiohttp.TCPConnector(limit=self.num_users, limit_per_host=self.num_users)
        timeout = aiohttp.ClientTimeout(total=10)
        
        async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
            # Create tasks for all users
            tasks = [
                self.user_session(i, session, endpoints)
                for i in range(self.num_users)
            ]
            
            # Run all user sessions concurrently
            await asyncio.gather(*tasks)
        
        total_time = time.time() - start_time
        
        # Print results
        self.print_results(total_time)
    
    def print_results(self, total_time):
        """Print test results"""
        print(f"\n{'='*60}")
        print(f"Load Test Results")
        print(f"{'='*60}")
        
        print(f"\n📊 Request Statistics:")
        print(f"  Total Requests: {self.results['total_requests']}")
        print(f"  Successful: {self.results['successful_requests']}")
        print(f"  Failed: {self.results['failed_requests']}")
        print(f"  Success Rate: {(self.results['successful_requests']/self.results['total_requests']*100):.2f}%")
        
        print(f"\n⏱️  Performance Metrics:")
        print(f"  Total Duration: {total_time:.2f} seconds")
        print(f"  Requests/Second: {self.results['total_requests']/total_time:.2f}")
        
        if self.results['response_times']:
            print(f"\n📈 Response Time Statistics (ms):")
            print(f"  Min: {min(self.results['response_times']):.2f}")
            print(f"  Max: {max(self.results['response_times']):.2f}")
            print(f"  Mean: {statistics.mean(self.results['response_times']):.2f}")
            print(f"  Median: {statistics.median(self.results['response_times']):.2f}")
            
            if len(self.results['response_times']) > 1:
                print(f"  Std Dev: {statistics.stdev(self.results['response_times']):.2f}")
                
            # Percentiles
            sorted_times = sorted(self.results['response_times'])
            p50 = sorted_times[len(sorted_times)//2]
            p95 = sorted_times[int(len(sorted_times)*0.95)]
            p99 = sorted_times[int(len(sorted_times)*0.99)]
            
            print(f"\n  Percentiles:")
            print(f"    P50: {p50:.2f} ms")
            print(f"    P95: {p95:.2f} ms")
            print(f"    P99: {p99:.2f} ms")
        
        print(f"\n🖥️  Server Distribution:")
        total_distributed = sum(self.results['server_distribution'].values())
        for server, count in sorted(self.results['server_distribution'].items()):
            percentage = (count / total_distributed * 100) if total_distributed > 0 else 0
            print(f"  {server}: {count} requests ({percentage:.2f}%)")
        
        print(f"\n📡 Status Codes:")
        for code, count in sorted(self.results['status_codes'].items()):
            print(f"  {code}: {count}")
        
        if self.results['errors']:
            error_summary = defaultdict(int)
            for error in self.results['errors']:
                error_summary[error] += 1
            
            print(f"\n❌ Error Summary (Top 5):")
            for error, count in sorted(error_summary.items(), key=lambda x: x[1], reverse=True)[:5]:
                print(f"  {error}: {count}")
        
        print(f"\n{'='*60}\n")
        
        # Save results to file
        with open('load_test_results.json', 'w') as f:
            results_to_save = {
                'total_requests': self.results['total_requests'],
                'successful_requests': self.results['successful_requests'],
                'failed_requests': self.results['failed_requests'],
                'server_distribution': dict(self.results['server_distribution']),
                'status_codes': dict(self.results['status_codes']),
                'response_time_stats': {
                    'min': min(self.results['response_times']) if self.results['response_times'] else 0,
                    'max': max(self.results['response_times']) if self.results['response_times'] else 0,
                    'mean': statistics.mean(self.results['response_times']) if self.results['response_times'] else 0,
                    'median': statistics.median(self.results['response_times']) if self.results['response_times'] else 0,
                },
                'duration': total_time,
                'requests_per_second': self.results['total_requests']/total_time
            }
            json.dump(results_to_save, f, indent=2)
        
        print("📄 Results saved to load_test_results.json")


async def main():
    # Configuration
    LOAD_BALANCER_URL = "http://localhost:8080"
    NUM_CONCURRENT_USERS = 1000
    TEST_DURATION = 60  # seconds
    
    print("\n🚀 Load Balancer Testing Tool")
    print("=" * 60)
    
    # Create and run load tester
    tester = LoadTester(
        base_url=LOAD_BALANCER_URL,
        num_users=NUM_CONCURRENT_USERS,
        duration=TEST_DURATION
    )
    
    await tester.run_test()


if __name__ == "__main__":
    asyncio.run(main())