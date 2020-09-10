use clap::Clap;
use std::fs::File;
use std::io::{self, BufReader};
use std::path::PathBuf;

use tf_record::{ReadRecordError, TfRecordState};

mod masked_crc;
mod tf_record;

#[derive(Clap)]
struct Opts {
    /// Event file to read.
    filename: PathBuf,

    /// Capacity override for `io::BufReader`.
    #[clap(long)]
    bufsize: Option<usize>,

    /// Validate TFRecord payloads against CRC-32 checksums.
    #[clap(long)]
    checksum: bool,
}

fn main() -> io::Result<()> {
    let opts = Opts::parse();
    let file = File::open(&opts.filename)?;
    let mut reader = match opts.bufsize {
        None => BufReader::new(file),
        Some(n) => BufReader::with_capacity(n, file),
    };

    let mut records_read = 0usize;
    let mut total_payload_size = 0usize;
    loop {
        let (payload_size, more) = process_one_record(&mut reader, &opts);
        records_read += 1;
        total_payload_size += payload_size;
        if !more {
            break;
        }
    }
    println!(
        "all done; read {} records ({} bytes payload)",
        records_read, total_payload_size
    );
    Ok(())
}

fn process_one_record<R: io::Read>(reader: &mut R, opts: &Opts) -> (usize, bool) {
    let mut state = TfRecordState::new();
    match tf_record::read_record(&mut state, reader) {
        Ok(record) => {
            if opts.checksum {
                match record.checksum() {
                    Ok(_) => (),
                    Err(e) => eprintln!("checksum failure: {:?}", e),
                }
            }
            (record.data.len(), true)
        }
        Err(ReadRecordError::Truncated) => return (0, false),
        Err(e) => {
            eprintln!("{:?}", e);
            std::process::exit(1);
        }
    }
}
